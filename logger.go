// Package logger provides a buffered, rotating logger with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
package logger

import (
	"context"
)

// Log level constants match slog levels for consistency with applications that use it.
// These values are used to determine which logs to write based on minimum level configuration.
const (
	LevelDebug int64 = -4 // matches slog.LevelDebug
	LevelInfo  int64 = 0  // matches slog.LevelInfo
	LevelWarn  int64 = 4  // matches slog.LevelWarn
	LevelError int64 = 8  // matches slog.LevelError
)

// Config defines the logger configuration parameters.
// All fields can be configured via JSON or TOML configuration files.
type Config struct {
	Level          int64  `json:"level" toml:"level"`                         // LevelDebug, LevelInfo, LevelWarn, LevelError
	Name           string `json:"name" toml:"name"`                           // Base name for log files
	Directory      string `json:"directory" toml:"directory"`                 // Directory to store log files
	BufferSize     int64  `json:"buffer_size" toml:"buffer_size"`             // Channel buffer size
	MaxSizeMB      int64  `json:"max_size_mb" toml:"max_size_mb"`             // Max size of each log file in MB
	MaxTotalSizeMB int64  `json:"max_total_size_mb" toml:"max_total_size_mb"` // Max total size of the log folder in MB to trigger old log deletion/pause logging
	MinDiskFreeMB  int64  `json:"min_disk_free_mb" toml:"min_disk_free_mb"`   // Min available free space in MB to trigger old log deletion/pause logging
	FlushTimer     int64  `json:"flush_timer" toml:"flush_timer"`             // Periodically forces writing logs to the disk to avoid missing logs on program shutdown
	TraceDepth     int64  `json:"trace_depth" toml:"trace_depth"`             // 0-10, 0 disables tracing
}

// Init initializes the logger with the provided configuration.
// It validates the configuration and sets up the logging infrastructure including file management and buffering.
func Init(ctx context.Context, cfg ...*Config) error {
	defaultConfig := &Config{
		Level:          LevelInfo,
		Name:           "log",
		Directory:      "./logs",
		BufferSize:     1024,
		MaxSizeMB:      10,
		MaxTotalSizeMB: 50,
		MinDiskFreeMB:  100,
		FlushTimer:     100,
		TraceDepth:     0,
	}

	if len(cfg) == 0 {
		return initLogger(ctx, defaultConfig)
	} else {
		userConfig := cfg[0]
		mergedCfg := &Config{
			Level:          getConfigValue(defaultConfig.Level, userConfig.Level),
			Name:           getConfigValue(defaultConfig.Name, userConfig.Name),
			Directory:      userConfig.Directory, // empty string is valid
			BufferSize:     getConfigValue(defaultConfig.BufferSize, userConfig.BufferSize),
			MaxSizeMB:      getConfigValue(defaultConfig.MaxSizeMB, userConfig.MaxSizeMB),
			MaxTotalSizeMB: getConfigValue(defaultConfig.MaxTotalSizeMB, userConfig.MaxTotalSizeMB),
			MinDiskFreeMB:  getConfigValue(defaultConfig.MinDiskFreeMB, userConfig.MinDiskFreeMB),
			FlushTimer:     getConfigValue(defaultConfig.FlushTimer, userConfig.FlushTimer),
			TraceDepth:     getConfigValue(defaultConfig.TraceDepth, userConfig.TraceDepth),
		}
		return initLogger(ctx, mergedCfg)
	}
}

// Debug logs a message at debug level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than debug or if logger is not initialized.
func Debug(logCtx context.Context, args ...any) {
	log(logCtx, LevelDebug, args...)
}

// Info logs a message at info level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than info or if logger is not initialized.
func Info(logCtx context.Context, args ...any) {
	log(logCtx, LevelInfo, args...)
}

// Warn logs a message at warning level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than warn or if logger is not initialized.
func Warn(logCtx context.Context, args ...any) {
	log(logCtx, LevelWarn, args...)
}

// Error logs a message at error level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than error or if logger is not initialized.
func Error(logCtx context.Context, args ...any) {
	log(logCtx, LevelError, args...)
}

// Shutdown gracefully shuts down the logger, ensuring all buffered messages are written
// and files are properly closed. It respects context cancellation for timeout control.
func Shutdown(ctx ...context.Context) error {
	shutdownCtx := context.Background()
	if len(ctx) > 0 {
		shutdownCtx = ctx[0]
	}
	return shutdownLogger(shutdownCtx)
}

// D logs a debug message without requiring context initialization.
// Message is dropped if logger's level is higher than debug.
func D(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelDebug, args...)
}

// I logs an info message without requiring context initialization.
// Message is dropped if logger's level is higher than info.
func I(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelInfo, args...)
}

// W logs a warning message without requiring context initialization.
// Message is dropped if logger's level is higher than warn.
func W(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelError, args...)
}

// E logs an error message without requiring context initialization.
// Message is dropped if logger's level is higher than error.
func E(args ...any) {
	if !ensureInitialized() {
		return
	}
	Error(context.Background(), args...)
}
