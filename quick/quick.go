package quick

import (
	"context"
	"github.com/LixenWraith/logger"
)

// Debug logs a debug message without requiring context initialization.
// Message is dropped if logger's level is higher than debug.
func Debug(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Debug(context.Background(), args...)
}

// Info logs an info message without requiring context initialization.
// Message is dropped if logger's level is higher than info.
func Info(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Info(context.Background(), args...)
}

// Warn logs a warning message without requiring context initialization.
// Message is dropped if logger's level is higher than warn.
func Warn(args ...any) {
	if !logger.EnsureInitialized() {
		return
	}
	logger.Warn(context.Background(), args...)
}

// Error logs an error message without requiring context initialization.
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

// Shutdown performs a graceful shutdown of the logger with a default timeout
func Shutdown() error {
	ctx := context.Background()
	return logger.Shutdown(ctx)
}