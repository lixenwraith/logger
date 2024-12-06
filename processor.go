// Package logger provides a buffered, rotating logger with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
// Lixen Wraith, 2024
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
)

// logRecord represents a single log entry with its complete context and metadata.
// It encapsulates all information needed to write a structured log entry.
type logRecord struct {
	LogCtx context.Context
	Level  int64
	Args   []any
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
func log(logCtx context.Context, level int64, depth int64, args ...any) {
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

		sendLogRecord(dropRecord)
	}

	logArgs := args
	// Get caller trace if set
	const skipTrace = 4 // 3 levels of logger calls + adjustment for runtime.Callers behavior

	if trace := getTrace(depth, skipTrace); trace != "" {
		logArgs = append([]any{trace}, args...)
	}

	// Create log record from arguments
	record := logRecord{
		LogCtx: logCtx,
		Level:  level,
		Args:   logArgs,
	}

	// Process log record
	sendLogRecord(record)
}

// sendLogRecord handles the safe sending of log records to the channel
func sendLogRecord(record logRecord) {
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