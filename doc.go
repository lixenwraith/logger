// Package logger provides a buffered, rotating logger with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
//
// Features:
// - Buffered asynchronous logging with configurable capacity
// - Automatic log file rotation based on size
// - Disk space management with configurable limits:
//   - Maximum total log directory size
//   - Minimum required free disk space
//   - Automatic cleanup of old logs when limits are reached
//
// - Dropped logs detection and reporting
// - Unique timestamp-based log file naming with nanosecond precision
// - Thread-safe operations using atomic counters
// - Context-aware logging with cancellation support
// - Multiple log levels (Debug, Info, Warn, Error) matching slog levels
// - Efficient JSON and TXT structured logging with zero allocations
// - Function call trace support with configurable depth
// - Graceful shutdown with context support
// - Runtime reconfiguration
// - Disk full protection with logging pause
// - Log retention management with configurable period and check interval
//
// Lixen Wraith, 2024
package logger