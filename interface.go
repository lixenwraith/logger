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

// Debug with trace
func DebugTrace(depth int, logCtx context.Context, args ...any) {
	log(logCtx, LevelDebug, int64(depth), args...)
}

// Info with trace
func InfoTrace(depth int, logCtx context.Context, args ...any) {
	log(logCtx, LevelInfo, int64(depth), args...)
}

// Warn with trace
func WarnTrace(depth int, logCtx context.Context, args ...any) {
	log(logCtx, LevelWarn, int64(depth), args...)
}

// Error with trace
func ErrorTrace(depth int, logCtx context.Context, args ...any) {
	log(logCtx, LevelError, int64(depth), args...)
}

// Config initializes the logger with the provided configuration.
func Config(cfg *LoggerConfig) error {
	return configLogger(context.Background(), cfg)
}

// D logs a debug message without requiring context initialization.
// Message is dropped if logger's level is higher than debug.
func D(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelDebug, traceDepth, args...)
}

// I logs an info message without requiring context initialization.
// Message is dropped if logger's level is higher than info.
func I(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelInfo, traceDepth, args...)
}

// W logs a warning message without requiring context initialization.
// Message is dropped if logger's level is higher than warn.
func W(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelWarn, traceDepth, args...)
}

// E logs an error message without requiring context initialization.
// Message is dropped if logger's level is higher than error.
func E(args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelError, traceDepth, args...)
}

// D with trace
func DT(depth int, args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelDebug, int64(depth), args...)
}

// I with trace
func IT(depth int, args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelInfo, int64(depth), args...)
}

// W with trace
func WT(depth int, args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelWarn, int64(depth), args...)
}

// E with trace
func ET(depth int, args ...any) {
	if !ensureInitialized() {
		return
	}
	log(context.Background(), LevelError, int64(depth), args...)
}