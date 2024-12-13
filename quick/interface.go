package quick

import (
	"context"
	"fmt"

	"github.com/LixenWraith/logger"
)

// Debug logs a debug message.
// Message is dropped if logger's level is higher than debug.
func Debug(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Debug(context.Background(), args...)
}

// Info logs an info message.
// Message is dropped if logger's level is higher than info.
func Info(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Info(context.Background(), args...)
}

// Warn logs a warning message.
// Message is dropped if logger's level is higher than warn.
func Warn(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Warn(context.Background(), args...)
}

// Error logs an error message.
// Message is dropped if logger's level is higher than error.
func Error(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Error(context.Background(), args...)
}

// DebugTrace is Debug log with trace.
func DebugTrace(depth int, args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.DebugTrace(context.Background(), depth, args...)
}

// InfoTrace is Info log with trace.
func InfoTrace(depth int, args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.InfoTrace(context.Background(), depth, args...)
}

// WarnTrace is Warning log with trace.
func WarnTrace(depth int, args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.WarnTrace(context.Background(), depth, args...)
}

// ErrorTrace is Error log with trace.
func ErrorTrace(depth int, args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.ErrorTrace(context.Background(), depth, args...)
}

// Log writes a log record without log level.
func Log(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.LogWithFlags(context.Background(), logger.FlagShowTimestamp, logger.LevelInfo, -1, args...)
}

// Log writes a log record with trace and without log level.
func LogTrace(depth int, args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.LogWithFlags(context.Background(), logger.FlagShowTimestamp, logger.LevelInfo, int64(depth), args...)
}

// Message writes a log record without timestamp and log level.
func Message(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.LogWithFlags(context.Background(), 0, logger.LevelInfo, 0, args...)
}

// Config changes the logger configuration with string statements.
// e.g. quick.Config("level=debug")
func Config(args ...string) error {
	if !logger.EnsureInitialized() {
		return fmt.Errorf("logger initialization failed")
	}

	if len(args) == 0 {
		return fmt.Errorf("no config provided")
	}

	cfg, err := config(args...)
	if err != nil {
		return err
	}

	return logger.Config(cfg)
}

// Shutdown performs a graceful shutdown of the logger with default default timeout
func Shutdown() {
	ctx := context.Background()
	_ = logger.Shutdown(ctx)
}