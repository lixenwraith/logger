package logger

import (
	"context"
	"fmt"
)

const (
	LevelDebug int = -4 // matches slog.LevelDebug
	LevelInfo  int = 0  // matches slog.LevelInfo
	LevelWarn  int = 4  // matches slog.LevelWarn
	LevelError int = 8  // matches slog.LevelError
)

type Config struct {
	Level      int    // LevelDebug, LevelInfo, LevelWarn, LevelError
	Name       string // Base name for log files
	Directory  string // Directory to store log files
	BufferSize int    // Channel buffer size
	MaxSizeMB  int64  // Max size of each log file in MB
}

func Init(ctx context.Context, cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("logger name cannot be empty")
	}

	return initLogger(ctx, cfg, cfg.Level)
}

func Debug(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelDebug < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelDebug, msg, args...)
}

func Info(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelInfo < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelInfo, msg, args...)
}

func Warn(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelWarn < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelWarn, msg, args...)
}

func Error(logCtx context.Context, msg string, args ...any) {
	if !isInitialized.Load() || LevelError < logLevel.Load().(int) {
		return
	}
	log(logCtx, LevelError, msg, args...)
}

func Shutdown(ctx context.Context) error {
	return shutdownLogger(ctx)
}
