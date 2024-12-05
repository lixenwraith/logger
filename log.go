// Package logger provides a buffered, rotating logger with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// logRecord represents a single log entry with its complete context and metadata.
// It encapsulates all information needed to write a structured log entry.
type logRecord struct {
	LogCtx context.Context
	Level  int64
	Args   []any
}

// Package level variables maintaining logger state and configuration.
// Thread-safety is ensured through atomic operations and mutex locks.
var (
	logChannel     chan logRecord
	isInitialized  atomic.Bool
	logLevel       atomic.Value // stores int64
	currentFile    atomic.Value // stores *os.File
	name           string
	directory      string
	maxSizeMB      int64
	maxTotalSizeMB int64
	minDiskFreeMB  int64
	flushTimer     time.Duration
	traceDepth     int64
	diskFullLogged atomic.Bool
	currentSize    atomic.Int64
	bufferSize     atomic.Int64
	droppedLogs    atomic.Uint64
	loggedDrops    atomic.Uint64
	processCtx     context.Context
	processCancel  context.CancelFunc
	mu             sync.RWMutex
	initMu         sync.Mutex
	loggerDisabled atomic.Bool
)

// shutdownOnce ensures the logger shutdown routine executes exactly once,
// even if multiple shutdown paths are triggered simultaneously.
var shutdownOnce sync.Once

// init sets up a finalizer to handle non-graceful program termination.
// It attempts to flush pending logs and close files without blocking program exit.
// Applications should still call Shutdown explicitly for graceful termination.
func init() {
	// Set finalizer for non-graceful exits
	runtime.SetFinalizer(&isInitialized, func(interface{}) {
		// Only attempt shutdown if logger was initialized
		if isInitialized.Load() {
			shutdownOnce.Do(func() {
				if err := Shutdown(context.Background()); err != nil {
					fmt.Fprintf(os.Stderr, "Logger shutdown error: %v\n", err)
				}
			})
		}
	})
}

// initLogger configures and starts the logging infrastructure with the provided configuration.
// It handles initialization of files, channels, and background processing while ensuring thread safety.
func initLogger(ctx context.Context, cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Setup directory and validate disk space configuration
		directory = cfg.Directory
		if directory == "" {
			directory = "."
		}
		if err := os.MkdirAll(directory, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		name = cfg.Name
		maxSizeMB = cfg.MaxSizeMB
		maxTotalSizeMB = cfg.MaxTotalSizeMB
		minDiskFreeMB = cfg.MinDiskFreeMB
		flushTimer = time.Duration(cfg.FlushTimer) * time.Millisecond

		newBufferSize := cfg.BufferSize
		if newBufferSize < 1 {
			newBufferSize = 1000
		}

		if maxTotalSizeMB < 0 || minDiskFreeMB < 0 {
			return fmt.Errorf("invalid disk space configuration")
		}

		if maxTotalSizeMB > 0 || minDiskFreeMB > 0 {
			if err := checkDiskSpace(ctx); err != nil {
				return fmt.Errorf("insufficient disk space for logging: %w", err)
			}
		}

		if cfg.TraceDepth < 0 || cfg.TraceDepth > 10 {
			return fmt.Errorf("invalid trace depth: must be between 0 and 10")
		}
		traceDepth = cfg.TraceDepth

		// Handle reconfiguration of existing logger
		if isInitialized.Load() {
			if processCancel != nil {
				processCancel()
			}

			logLevel.Store(cfg.Level)

			if newBufferSize != bufferSize.Load() {
				oldChannel := logChannel
				logChannel = make(chan logRecord, newBufferSize)
				bufferSize.Store(newBufferSize)
				close(oldChannel)
			}
		}

		// Initialize new log file and logger instance
		logFile, err := createNewLogFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create initial log file: %w", err)
		}

		currentFile.Store(logFile)
		logLevel.Store(cfg.Level)
		bufferSize.Store(newBufferSize)

		logChannel = make(chan logRecord, newBufferSize)

		processCtx, processCancel = context.WithCancel(ctx)
		go processLogs()

		isInitialized.Store(true)
		return nil
	}
}

// log handles the actual logging operation including dropped log detection and disk space checks.
// It buffers log records through a channel for asynchronous processing.
func log(logCtx context.Context, level int64, args ...any) {
	// Check if logger is initialized and if log should be processed based on level
	if !isInitialized.Load() {
		return
	}
	if level < logLevel.Load().(int64) {
		return
	}

	// Check disk space before attempting to log
	if err := checkDiskSpace(logCtx); err != nil {
		droppedLogs.Add(1)
		return
	}

	// Process any dropped logs before handling new log
	currentDrops := droppedLogs.Load()
	logged := loggedDrops.Load()
	if currentDrops > logged {
		// Immediately update the logged drop counter to
		// current dropped log counter to avoid conflict.
		loggedDrops.Store(currentDrops)
		dropRecord := logRecord{
			LogCtx: context.Background(),
			Level:  LevelError,
			Args: []any{
				"Logs were dropped",
				"dropped_count", currentDrops - logged,
				"total_dropped", currentDrops,
			},
		}

		select {
		case logChannel <- dropRecord:
		default:
			droppedLogs.Add(1)
		}
	}

	logArgs := args
	// Get caller trace if set
	const skipTrace = 4 // 3 levels of logger calls + adjustment for runtime.Callers behavior

	if trace := getTrace(skipTrace); trace != "" {
		logArgs = append([]any{trace}, args...)
	}

	// Create log record from arguments
	record := logRecord{
		LogCtx: logCtx,
		Level:  level,
		Args:   logArgs,
	}

	// Process log record
	select {
	case logChannel <- record:
	default:
		droppedLogs.Add(1)
	}
}

// processLogs is the main log processing loop running in a separate goroutine.
// It handles the actual writing of logs and manages file rotation based on size.
func processLogs() {
	ticker := time.NewTicker(flushTimer)
	defer ticker.Stop()

	for {
		select {
		// Process each log record
		case record, ok := <-logChannel:
			if !ok {
				if currentFile := currentFile.Load().(*os.File); currentFile != nil {
					currentFile.Sync()
				}
				return
			}

			// Create log entry and write
			s := newSerializer()
			data := s.serialize(record.Level, record.Args)

			// Check file size and rotate if needed
			currentFileSize := currentSize.Load()
			estimatedSize := currentFileSize + int64(len(data))

			if maxSizeMB > 0 && estimatedSize > maxSizeMB*1024*1024 {
				if err := rotateLogFile(record.LogCtx); err != nil {
					continue
				}
			}

			if _, err := currentFile.Load().(*os.File).Write(data); err != nil {
				continue
			}

			// Sync after each write during shutdown
			if !isInitialized.Load() {
				currentFile.Load().(*os.File).Sync()
			}

			if fi, err := os.Stat(currentFile.Load().(*os.File).Name()); err == nil {
				currentSize.Store(fi.Size())
			}
		case <-ticker.C:
			if currentFile := currentFile.Load().(*os.File); currentFile != nil {
				currentFile.Sync()
			}
		case <-processCtx.Done():
			if currentFile := currentFile.Load().(*os.File); currentFile != nil {
				currentFile.Sync()
			}
			return
		}
	}
}

// generateLogFileName creates a unique log filename using timestamp with increasing precision.
// It ensures uniqueness by progressively adding more precise subsecond components.
func shutdownLogger(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if !isInitialized.Load() {
		return nil
	}

	loggerDisabled.Store(true) // Prevent new logs/reinit but keep processing
	time.Sleep(2 * flushTimer)

	if processCancel != nil {
		processCancel()
	}
	close(logChannel)

	if currentFile := currentFile.Load().(*os.File); currentFile != nil {
		if err := currentFile.Sync(); err != nil { // final sync before closing
			return fmt.Errorf("failed to sync log file: %w", err)
		}
		if err := currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
	}

	isInitialized.Store(false)
	return nil
}

// generateLogFileName creates a unique log filename using timestamp with increasing precision.
// It ensures uniqueness by progressively adding more precise subsecond components.
func generateLogFileName(ctx context.Context, baseName string, timestamp time.Time) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		baseTimestamp := timestamp.Format("060102_150405")
		// Always include first decimal place (tenth of a second)
		tenths := (timestamp.UnixNano() % 1e9) / 1e8
		filename := fmt.Sprintf("%s_%s.%d.log", baseName, baseTimestamp, tenths)
		fullPath := filepath.Join(directory, filename)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return filename, nil
		}

		// If file exists, try additional precision
		for precision := 2; precision <= 9; precision++ {
			subseconds := timestamp.UnixNano() % 1e9 / pow10(9-precision)
			subsecFormat := fmt.Sprintf("%%0%dd", precision)
			filename = fmt.Sprintf("%s_%s_%s.log",
				baseName,
				baseTimestamp,
				fmt.Sprintf(subsecFormat, subseconds))

			fullPath = filepath.Join(directory, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return filename, nil
			}
		}
		return "", fmt.Errorf("failed to generate unique log filename")
	}
}

// pow10 calculates powers of 10 for subsecond precision in log filenames.
// It's used by generateLogFileName to create unique timestamp-based names.
func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// createNewLogFile generates and opens a new log file with proper permissions.
// It ensures unique naming and proper file creation with append mode.
func createNewLogFile(ctx context.Context) (*os.File, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		filename, err := generateLogFileName(ctx, name, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to generate log filename: %w", err)
		}

		file, err := os.OpenFile(
			filepath.Join(directory, filename),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create log file: %w", err)
		}
		return file, nil
	}
}

// rotateLogFile handles the log rotation process, creating new file and closing old one.
// It updates all necessary state while maintaining thread safety.
func rotateLogFile(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		newFile, err := createNewLogFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create new log file: %w", err)
		}

		oldFile := currentFile.Load().(*os.File)
		if oldFile != nil {
			if err := oldFile.Close(); err != nil {
				newFile.Close()
				return fmt.Errorf("failed to close old log file: %w", err)
			}
		}

		currentFile.Store(newFile)
		currentSize.Store(0)

		return nil
	}
}

// getDiskStats retrieves filesystem statistics for the log directory.
// It returns available and total space in bytes.
func getDiskFreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks * block size
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// getLogDirSize calculates total size of all log files in the directory.
// It only counts files with .log extension.
func getLogDirSize(dir string) (int64, error) {
	var size int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.IsDir() && filepath.Ext(entry.Name()) == ".log" {
			size += info.Size()
		}
	}
	return size, nil
}

// cleanOldLogs removes oldest log files to free up required disk space.
// It sorts files by modification time and removes them until enough space is freed.
func cleanOldLogs(ctx context.Context, required int64) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	// Build list of log files with their metadata
	type logFile struct {
		name    string
		modTime time.Time
		size    int64
	}

	var logs []logFile
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".log" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if f := currentFile.Load().(*os.File); f != nil &&
			entry.Name() == filepath.Base(f.Name()) {
			continue
		}
		logs = append(logs, logFile{
			name:    entry.Name(),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}
	if len(logs) == 0 {
		return fmt.Errorf("no logs available to delete, needed %d bytes", required)
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].modTime.Before(logs[j].modTime)
	})

	var deleted int64
	for _, log := range logs {
		if deleted >= required {
			break
		}
		if err := os.Remove(filepath.Join(directory, log.name)); err != nil {
			continue
		}
		deleted += log.size
	}
	if deleted < required {
		return fmt.Errorf("could not free enough space: freed %d bytes, needed %d bytes", deleted, required)
	}
	return nil
}

// checkDiskSpace ensures sufficient disk space is available for logging.
// It manages disk space by cleaning up old logs and pausing logging if necessary.
func checkDiskSpace(ctx context.Context) error {
	// Skip check if disk management not configured
	if maxTotalSizeMB == 0 && minDiskFreeMB == 0 {
		return nil
	}

	// Check current disk space and directory size
	free, err := getDiskFreeSpace(directory)
	if err != nil {
		return err
	}

	dirSize, err := getLogDirSize(directory)
	if err != nil {
		return err
	}

	minFree := minDiskFreeMB * 1024 * 1024
	maxTotal := maxTotalSizeMB * 1024 * 1024

	if free < minFree || (maxTotal > 0 && dirSize > maxTotal) {
		required := int64(0)
		if free < minFree {
			required = minFree - free
		}
		if maxTotal > 0 && dirSize > maxTotal {
			exceeded := dirSize - maxTotal
			if exceeded > required {
				required = exceeded
			}
		}

		if err := cleanOldLogs(ctx, required); err != nil {
			if !diskFullLogged.Load() {
				diskFullLogged.Store(true)
				return fmt.Errorf("disk full")
			}
		}
	}

	diskFullLogged.Store(false)
	return nil
}

// stringifyMessage converts any type to a string representation
func stringifyMessage(msg any) string {
	switch m := msg.(type) {
	case string:
		return m
	case error:
		return m.Error()
	case fmt.Stringer:
		return m.String()
	default:
		return fmt.Sprintf("%+v", m)
	}
}

// ensureInitialized checks if logger is initialized and initializes with defaults if needed
func ensureInitialized() bool {
	// If previous initialization failed, drop logs silently
	if loggerDisabled.Load() {
		return false
	}

	// If already initialized successfully, proceed with logging
	if isInitialized.Load() {
		return true
	}

	// Try to initialize
	initMu.Lock()
	defer initMu.Unlock()

	// Double check both conditions after lock
	if loggerDisabled.Load() || isInitialized.Load() {
		return isInitialized.Load()
	}

	if err := Init(context.Background()); err != nil {
		// Mark initialization as failed and silently drop future logs
		loggerDisabled.Store(true)
		return false
	}

	return true
}

func getTrace(skip int) string {
	depth := int(traceDepth)
	if depth == 0 {
		return ""
	}

	// Capture up to depth+skip frames to account for internal calls
	pc := make([]uintptr, depth+skip)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return "(unknown)"
	}

	frames := runtime.CallersFrames(pc[:n])
	var trace []string
	count := 0

	for {
		frame, more := frames.Next()
		if !more || count >= depth {
			break
		}

		funcName := filepath.Base(frame.Function)
		if strings.HasPrefix(funcName, "func") {
			funcName = fmt.Sprintf("(anonymous %s)", funcName)
		}
		trace = append(trace, funcName)
		count++
	}

	if len(trace) == 0 {
		return "(unknown)"
	}

	// Reverse the trace array before joining for outer -> inner order
	for i := 0; i < len(trace)/2; i++ {
		j := len(trace) - i - 1
		trace[i], trace[j] = trace[j], trace[i]
	}
	return strings.Join(trace, " -> ")
}

// getConfigValue returns defaultVal if cfgVal equals the zero value for type T,
// otherwise returns cfgVal. Type T must satisfy the comparable constraint.
// This is commonly used for merging configuration values with their defaults.
func getConfigValue[T comparable](defaultVal, cfgVal T) T {
	var zero T
	if cfgVal == zero {
		return defaultVal
	}
	return cfgVal
}
