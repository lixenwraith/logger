// Package logger provides a buffered, rotating logger with production-ready features
// including automatic file rotation, disk space management, and dropped log detection.
//
// Features:
//   - Buffered asynchronous logging with configurable capacity
//   - Automatic log file rotation based on size
//   - Disk space management with configurable limits
//   - Dropped logs detection and reporting
//   - Unique timestamp-based log file naming
//   - Thread-safe operations
//   - Context-aware logging
//   - Multiple log levels matching slog levels
//   - Function call trace support
//   - Graceful shutdown
//
// Lixen Wraith, 2024
package logger