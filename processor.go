package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

// Context, channel, buffer, processing vars
var (
	processCtx    context.Context
	processCancel context.CancelFunc

	logChannel chan logRecord
	bufferSize atomic.Int64

	droppedLogs atomic.Uint64
	loggedDrops atomic.Uint64

	flushTimer time.Duration
	traceDepth int64

	flags int64
)

const (
	// Record flags for controlling output structure
	FlagShowTimestamp int64 = 0b01
	FlagShowLevel     int64 = 0b10
	FlagDefault             = FlagShowTimestamp | FlagShowLevel
)

// logRecord represents a single log entry with its complete context and metadata.
// It encapsulates all information needed to write a structured log entry.
type logRecord struct {
	LogCtx    context.Context
	Flags     int64
	TimeStamp time.Time
	Level     int64
	Trace     string
	Args      []any
}

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

// log handles the actual logging operation including dropped log detection and disk space checks.
// It buffers log records through a channel for asynchronous processing.
func log(logCtx context.Context, flags int64, level int64, depth int64, args ...any) {
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
			LogCtx:    context.Background(),
			Flags:     FlagDefault,
			TimeStamp: time.Now(),
			Level:     LevelError,
			Args: []any{
				"Logs were dropped",
				"dropped_count", currentDrops - logged,
				"total_dropped", currentDrops,
			},
		}

		sendLogRecord(dropRecord)
	}

	logArgs := args
	// Get caller trace if set
	const skipTrace = 4 // 3 levels of logger calls + adjustment for runtime.Callers behavior

	var trace string
	if depth > 0 {
		trace = getTrace(depth, skipTrace)
	}

	// Create log record from arguments
	record := logRecord{
		LogCtx:    logCtx,
		Flags:     flags,
		TimeStamp: time.Now(),
		Level:     level,
		Trace:     trace,
		Args:      logArgs,
	}

	// Process log record
	sendLogRecord(record)
}

// sendLogRecord handles the safe sending of log records to the channel
func sendLogRecord(record logRecord) {
	// mainly to handle shutdown when goroutines write to closed channel
	defer func() {
		if recover() != nil {
			droppedLogs.Add(1)
		}
	}()

	if loggerDisabled.Load() {
		droppedLogs.Add(1)
		return
	}

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

	var retentionTicker *time.Ticker
	var retentionChan <-chan time.Time // nil channel
	if retentionPeriod > 0 && retentionCheck > 0 {
		retentionTicker = time.NewTicker(retentionCheck)
		defer retentionTicker.Stop()
		retentionChan = retentionTicker.C // assign channel only if ticker exists
		updateEarliestFileTime()
	}

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
			data := s.serialize(record.Flags, record.TimeStamp, record.Level, record.Trace, record.Args)

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
		case <-retentionChan:
			// Only process if retention is enabled
			if retentionPeriod > 0 {
				// Safe type assertion and non-zero check
				if earliest, ok := earliestFileTime.Load().(time.Time); ok {
					// Only process if we have a valid timestamp
					if !earliest.IsZero() && time.Since(earliest) > retentionPeriod {
						ctx := context.Background()
						if err := cleanExpiredLogs(ctx, earliest); err == nil {
							// Only update if cleanup succeeded
							updateEarliestFileTime()
						}
					}
				}
			}
		case <-processCtx.Done():
			if currentFile := currentFile.Load().(*os.File); currentFile != nil {
				currentFile.Sync()
			}
			return
		}
	}
}

// getTrace returns a function call trace as a string, formatted as "outer -> inner -> deepest".
// It skips the specified number of frames and captures up to depth levels of function calls.
// Returns empty string if depth is 0, or "(unknown)" if no frames are captured.
// Function names are simplified to base names, with special handling for anonymous functions.
func getTrace(depth int64, skip int) string {
	if depth == 0 {
		return ""
	}

	// Capture up to depth+skip frames to account for internal calls
	pc := make([]uintptr, int(depth)+skip)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return "(unknown)"
	}

	frames := runtime.CallersFrames(pc[:n])
	var trace []string
	count := 0

	for {
		frame, more := frames.Next()
		if !more || count >= int(depth) {
			break
		}

		funcName := filepath.Base(frame.Function)
		parts := strings.Split(funcName, ".")
		lastPart := parts[len(parts)-1]
		if strings.HasPrefix(lastPart, "func") {
			// Check if rest is just digits
			afterFunc := lastPart[4:]
			isAnonymous := true
			for _, c := range afterFunc {
				if !unicode.IsDigit(c) {
					isAnonymous = false
					break
				}
			}
			if isAnonymous {
				funcName = fmt.Sprintf("(anonymous %s)", funcName)
			}
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

// EnsureInitialized checks if logger is initialized and initializes with defaults if needed
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