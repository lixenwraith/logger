package logger

import "context"

// Log level constants match slog levels for consistency with applications that use it.
// These values are used to determine which logs to write based on minimum level configuration.
const (
	LevelDebug int64 = -4 // matches slog.LevelDebug
	LevelInfo  int64 = 0  // matches slog.LevelInfo
	LevelWarn  int64 = 4  // matches slog.LevelWarn
	LevelError int64 = 8  // matches slog.LevelError
)

// Init initializes the logger with the provided configuration and context.
func Init(ctx context.Context, cfg ...*LoggerConfig) error {
	return configLogger(ctx, cfg...)
}

// Debug logs a message at debug level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than debug or if logger is not initialized.
func Debug(logCtx context.Context, args ...any) {
	log(logCtx, LevelDebug, traceDepth, args...)
}

// Info logs a message at info level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than info or if logger is not initialized.
func Info(logCtx context.Context, args ...any) {
	log(logCtx, LevelInfo, traceDepth, args...)
}

// Warn logs a message at warning level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than warn or if logger is not initialized.
func Warn(logCtx context.Context, args ...any) {
	log(logCtx, LevelWarn, traceDepth, args...)
}

// Error logs a message at error level with the given context and additional arguments.
// Messages are dropped if the logger's level is higher than error or if logger is not initialized.
func Error(logCtx context.Context, args ...any) {
	log(logCtx, LevelError, traceDepth, args...)
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

// DebugTrace is Debug log with trace.
func DebugTrace(logCtx context.Context, depth int, args ...any) {
	log(logCtx, LevelDebug, int64(depth), args...)
}

// InfoTrace is Info log with trace.
func InfoTrace(logCtx context.Context, depth int, args ...any) {
	log(logCtx, LevelInfo, int64(depth), args...)
}

// WarnTrace is Warn log with trace.
func WarnTrace(logCtx context.Context, depth int, args ...any) {
	log(logCtx, LevelWarn, int64(depth), args...)
}

// ErrorTrace is Error log with trace.
func ErrorTrace(logCtx context.Context, depth int, args ...any) {
	log(logCtx, LevelError, int64(depth), args...)
}

// Config initializes the logger with the provided configuration.
func Config(cfg *LoggerConfig) error {
	return configLogger(context.Background(), cfg)
}

// EnsureInitialized checks if the logger is initialized, and initializes if not.
// returns true if it was already initialized or initialization attempt was successful.
// returns false if logger cannot be initialized.
func EnsureInitialized() bool {
	return ensureInitialized()
}