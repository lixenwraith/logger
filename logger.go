// Package logger provides a buffered, rotating logger that wraps slog with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
package logger

import (
	"context"
	"fmt"
)

// Log level constants matching slog levels for consistent logging across the application.
// These values are used to determine which logs to write based on minimum level configuration.
const (
	LevelDebug int = -4 // matches slog.LevelDebug
	LevelInfo  int = 0  // matches slog.LevelInfo
	LevelWarn  int = 4  // matches slog.LevelWarn
	LevelError int = 8  // matches slog.LevelError
)

// Config defines the logger configuration parameters.
// All fields can be configured via JSON or TOML configuration files.
type Config struct {
	Level          int    `json:"level" toml:"level"`                         // LevelDebug, LevelInfo, LevelWarn, LevelError
	Name           string `json:"name" toml:"name"`                           // Base name for log files
	Directory      string `json:"directory" toml:"directory"`                 // Directory to store log files
	BufferSize     int    `json:"buffer_size" toml:"buffer_size"`             // Channel buffer size
	MaxSizeMB      int64  `json:"max_size_mb" toml:"max_size_mb"`             // Max size of each log file in MB
	MaxTotalSizeMB int64  `json:"max_total_size_mb" toml:"max_total_size_mb"` // Max total size of the log folder in MB to trigger old log deletion/pause logging
	MinDiskFreeMB  int64  `json:"min_disk_free_mb" toml:"min_disk_free_mb"`   // Min available free space in MB to trigger old log deletion/pause logging
}

// Init initializes the logger with the provided configuration.
// It validates the configuration and sets up the logging infrastructure including file management and buffering.
func Init(ctx context.Context, cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("logger name cannot be empty")
	}
	return initLogger(ctx, cfg, cfg.Level)
}

// Debug logs a message at debug level with the given context and key-value pairs.
// Messages are dropped if the logger's level is higher than debug or if logger is not initialized.
func Debug(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelDebug < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelDebug, msg, args...)
}

// Info logs a message at info level with the given context and key-value pairs.
// Messages are dropped if the logger's level is higher than info or if logger is not initialized.
func Info(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelInfo < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelInfo, msg, args...)
}

// Warn logs a message at warning level with the given context and key-value pairs.
// Messages are dropped if the logger's level is higher than warn or if logger is not initialized.
func Warn(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelWarn < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelWarn, msg, args...)
}

// Error logs a message at error level with the given context and key-value pairs.
// Messages are dropped if the logger's level is higher than error or if logger is not initialized.
func Error(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelError < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelError, msg, args...)
}

// Shutdown gracefully shuts down the logger, ensuring all buffered messages are written
// and files are properly closed. It respects context cancellation for timeout control.
func Shutdown(ctx context.Context) error {
	return shutdownLogger(ctx)
}
