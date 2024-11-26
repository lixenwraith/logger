// Package logger provides a buffered, rotating logger that wraps slog with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// logRecord represents a single log entry with its complete context and metadata.
// It encapsulates all information needed to write a structured log entry.
type logRecord struct {
	LogCtx  context.Context
	Level   int
	Message string
	Args    []any
	Time    time.Time
}

// Package level variables maintaining logger state and configuration.
// Thread-safety is ensured through atomic operations and mutex locks.
var (
	logChannel     chan logRecord
	isInitialized  atomic.Bool
	logLevel       atomic.Value // stores int
	logger         atomic.Value // stores *slog.Logger
	currentFile    atomic.Value // stores *os.File
	name           string
	directory      string
	maxSizeMB      int64
	maxTotalSizeMB int64
	minDiskFreeMB  int64
	diskFullLogged atomic.Bool
	currentSize    atomic.Int64
	bufferSize     atomic.Int64
	droppedLogs    atomic.Uint64
	loggedDrops    atomic.Uint64
	processCtx     context.Context
	processCancel  context.CancelFunc
	mu             sync.RWMutex
)

// initLogger configures and starts the logging infrastructure with the provided configuration.
// It handles initialization of files, channels, and background processing while ensuring thread safety.
func initLogger(ctx context.Context, cfg *Config, level int) error {
	mu.Lock()
	defer mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Setup directory and validate disk space configuration
		directory = cfg.Directory
		if err := os.MkdirAll(directory, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		name = cfg.Name
		maxSizeMB = cfg.MaxSizeMB
		maxTotalSizeMB = cfg.MaxTotalSizeMB
		minDiskFreeMB = cfg.MinDiskFreeMB

		newBufferSize := int64(cfg.BufferSize)
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

		// Handle reconfiguration of existing logger
		if isInitialized.Load() {
			if processCancel != nil {
				processCancel()
			}

			logLevel.Store(level)

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
		logLevel.Store(level)
		bufferSize.Store(newBufferSize)

		logger.Store(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: slog.Level(logLevel.Load().(int)),
		})))

		logChannel = make(chan logRecord, newBufferSize)

		processCtx, processCancel = context.WithCancel(ctx)
		go processLogs()

		isInitialized.Store(true)
		return nil
	}
}

// log handles the actual logging operation including dropped log detection and disk space checks.
// It buffers log records through a channel for asynchronous processing.
func log(logCtx context.Context, level int, msg string, args ...any) {
	// Check disk space before attempting to log
	if err := checkDiskSpace(logCtx); err != nil {
		droppedLogs.Add(1)
		return
	}

	record := logRecord{
		LogCtx:  logCtx,
		Level:   level,
		Message: msg,
		Args:    args,
		Time:    time.Now(),
	}
	// Process any dropped logs before handling new log
	currentDrops := droppedLogs.Load()
	logged := loggedDrops.Load()
	if currentDrops > logged {
		// Check disk space and buffer space first
		select {
		case <-logCtx.Done():
			return
		default:
			// Immediately update the logged drop counter to current dropped log counter
			// to avoid conflict.
			loggedDrops.Store(currentDrops)

			record := logRecord{
				LogCtx:  logCtx,
				Level:   LevelError,
				Message: "Logs were dropped",
				Args: []any{
					"dropped_count", currentDrops - logged,
					"total_dropped", currentDrops,
				},
				Time: time.Now(),
			}

			select {
			case logChannel <- record:
			default:
				droppedLogs.Add(1)
			}
		}
	}

	// Process log record
	select {
	case logChannel <- record:
	case <-logCtx.Done():
		droppedLogs.Add(1)
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

	if processCancel != nil {
		processCancel()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		close(logChannel)

		if currentFile := currentFile.Load().(*os.File); currentFile != nil {
			if err := currentFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}

		isInitialized.Store(false)
		return nil
	}
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

		logger.Store(slog.New(slog.NewJSONHandler(newFile, &slog.HandlerOptions{
			Level: slog.Level(logLevel.Load().(int)),
		})))

		return nil
	}
}

// processLogs is the main log processing loop running in a separate goroutine.
// It handles the actual writing of logs and manages file rotation based on size.
func processLogs() {
	for {
		select {
		case <-processCtx.Done():
			return
		// Process each log record
		case record, ok := <-logChannel:
			if !ok {
				return
			}

			select {
			case <-record.LogCtx.Done():
				continue
			default:
				log := logger.Load().(*slog.Logger)
				attrs := []slog.Attr{
					slog.Time("timestamp", record.Time),
					slog.Any("args", record.Args),
				}

				// Check file size and rotate if needed
				currentFileSize := currentSize.Load()
				estimatedSize := currentFileSize + 512

				if maxSizeMB > 0 && estimatedSize > maxSizeMB*1024*1024 {
					if err := rotateLogFile(record.LogCtx); err != nil {
						continue
					}
				}

				log.LogAttrs(record.LogCtx, slog.Level(record.Level), record.Message, attrs...)

				if fi, err := os.Stat(currentFile.Load().(*os.File).Name()); err == nil {
					currentSize.Store(fi.Size())
				}
			}
		}
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
