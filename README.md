# Logger

A buffered rotating logger for Go with advanced disk management, retention, and tracing capabilities.

## Features

- Buffered asynchronous logging with configurable capacity
- Automatic log file rotation based on size
- Disk space management with configurable limits:
    - Maximum total log directory size
    - Minimum required free disk space
    - Automatic cleanup of old logs when limits are reached
- Dropped logs detection and reporting
- Unique timestamp-based log file naming with nanosecond precision
- Thread-safe operations using atomic counters
- Context-aware logging with cancellation support
- Multiple log levels (Debug, Info, Warn, Error) matching slog levels
- Efficient JSON and TXT structured logging with zero allocations
- Function call trace support with configurable depth
- Graceful shutdown with context support
- Runtime reconfiguration
- Disk full protection with logging pause
- Log retention management with configurable period and check interval

## Installation

```bash
go get github.com/LixenWraith/logger
```

## Quick Start

```go
package main

import (
	"context"
	"github.com/LixenWraith/logger"
)

func main() {
	// config is optional, partial or no config is acceptable
	// unconfigured parameters use default values
	cfg := &logger.LoggerConfig{
		Level:                  logger.LevelInfo,
		Name:                   "myapp",
		Directory:              "/var/log/myapp",
		Format:                 "json",   // "txt" or "json", defaults to "txt"
		Extension:              "app",    // log file extension (default "log", empty = use format)
		BufferSize:             1000,     // log channel buffers 1000 log records
		MaxSizeMB:              100,      // Rotate files at 100MB
		MaxTotalSizeMB:         1000,     // Keep total logs under 1GB
		MinDiskFreeMB:          500,      // Require 500MB free space
		FlushTimer:             1000,     // Force writing to disk every 1 second
		TraceDepth:             2,        // Include 2 levels of function calls in trace
		RetentionPeriod:        7.0 * 24, // Keep logs for 7 days
		RetentionCheckInterval: 2.0 * 60, // Check every 2 hours
	}

	ctx := context.Background()
	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	logger.Info(ctx, "Application started", "version", "1.0.0")
}

```

## Configuration

The `LoggerConfig` struct provides the following options:

| Option                 | Description                                           | Default   |
|------------------------|-------------------------------------------------------|-----------|
| Level                  | Minimum log level to record                           | LevelInfo |
| Name                   | Base name for log files                               | log       |
| Directory              | Directory to store log files                          | ./logs    |
| Format                 | Log file format ("txt", "json")                       | "txt"     |
| Extension              | Log file extension (default: .log)                    | "log"     |
| ShowTimestamp          | Show timestamp in log entries                         | true      |
| ShowLevel              | Show log level in entries                             | true      |
| BufferSize             | Channel buffer size for burst handling                | 1024      |
| MaxSizeMB              | Maximum size of each log file before rotation         | 10        |
| MaxTotalSizeMB         | Maximum total size of log directory (0 disables)      | 50        |
| MinDiskFreeMB          | Minimum required free disk space (0 disables)         | 100       |
| FlushTimer             | Time in milliseconds to force writing to disk         | 100       |
| TraceDepth             | Number of function calls to include in trace (max 10) | 0         |
| RetentionPeriod        | Hours to keep log files (0 disables)                  | 0.0       |
| RetentionCheckInterval | Minutes between retention checks                      | 60.0      |

## Disk Space Management

The logger automatically manages disk space through several mechanisms:

- Rotates individual log files when they reach MaxSizeMB
- Monitors total log directory size against MaxTotalSizeMB
- Tracks available disk space against MinDiskFreeMB
- When limits are reached:
    1. Attempts to delete oldest log files first
    2. Pauses logging if space cannot be freed
    3. Resumes logging when space becomes available
    4. Records dropped logs during paused periods
- Automatically removes logs older (based on modification date) than RetentionPeriod if enabled

## Usage

### Logging Methods

with context (requires initialization).

```go
logger.Init(ctx, cfg)
logger.Debug(ctx, "Debug message", "key", "value", "state")
logger.Info(ctx, "Info message", "user", userID)
logger.Warn(ctx, "Warning message", "latency_ms", 150)
logger.Error(ctx, "Error message", err.Error())
err := logger.Shutdown(ctx)
```

simplified, doesn't need initialization (uses default config).
clean shutdown is recommended.

```go
quick.Info("Starting app")
quick.Error(err, "operation", "db_connect", "retry", 3)
quick.Warn(customError{}, "component", "cache")
quick.Debug(response, "request_id", reqID)
quick.Shutdown() // to ensure all logs are written if the program finishes before logs are flushed to disk
```

### Runtime Reconfiguration

The logger supports live reconfiguration while preserving existing logs.
Note that reconfiguration starts a new log file.

```go
newCfg := &logger.LoggerConfig{
Level:                  logger.LevelDebug,
Name:                   "myapp",
Directory:              "/new/log/path",
Format:                 "json",
Extension:              "json",
BufferSize:             2000,
MaxSizeMB:              200,
MaxTotalSizeMB:         2000,
MinDiskFreeMB:          1000,
FlushTimer:             50,
TraceDepth:             5,
RetentionPeriod:        24 * 30.0,
RetentionCheckInterval: 24 * 60.0,
}

if err := logger.Init(ctx, newCfg); err != nil {
// Handle error
}
```

Quick configuration is also available using key=value strings.
Keys follow toml/json tag names of the LoggerConfig.

```go
if err := quick.Config(
"level=debug",
"format=json",
"max_size_mb=100",
); err != nil {
// Handle error
}
```

### Function Call Tracing

The logger supports automatic function call tracing with configurable depth:

```go
cfg := &logger.LoggerConfig{
TraceDepth: 3, // Capture up to 3 levels of function calls
}
```

When enabled, each log entry will include the function call chain that led to the logging call. This helps with
debugging and understanding the code flow. The trace depth can be set between 0 (disabled/no trace) and 10 levels.
Example output with TraceDepth=2:

```json
{
  "time": "2024-03-21T15:04:05.123456789Z",
  "level": "INFO",
  "fields": [
    "main.processOrder -> main.validateInput",
    "Order validated",
    "order_id",
    "12345"
  ]
}
```

### Temporary Function Call Tracing

While the logger configuration supports persistent function call tracing, it can also be enabled for specific log
entries using trace variants of logging functions:

```go
// Context-aware logging with trace
logger.InfoTrace(ctx, 3, "Processing order", "id", orderId) // Shows 3 levels of function calls
logger.DebugTrace(ctx, 2, "Cache miss", "key", cacheKey) // Shows 2 levels
logger.WarnTrace(ctx, 4, "Retry attempt", "count", retries) // Shows 4 levels
logger.ErrorTrace(ctx, 5, "Operation failed", "error", err) // Shows 5 levels

// Simplified logging with trace
quick.InfoTrace(3, "Worker started", "pool", poolID) // Info with 3 levels
quick.DebugTrace(2, "Request received")              // Debug with 2 levels
quick.WarnTrace(4, "Connection timeout")             // Warning with 4 levels
quick.ErrorTrace(5, err, "Database error") // Error with 5 levels
```

These functions temporarily override the configured TraceDepth for a single log entry. This is useful for debugging
specific code paths without enabling tracing for all logs:

```go
func processOrder(ctx context.Context, orderID string) {
// Normal log without trace
logger.Info(ctx, "Processing started", "order", orderID)

if err := validate(orderID); err != nil {
// Log error with function call trace
logger.ErrorTrace(ctx, 3, "Validation failed", "error", err)
return
}

// Back to normal logging
logger.Info(ctx, "Processing completed", "order", orderID)
}
```

The trace depth parameter works the same way as the TraceDepth configuration option, accepting values from 0 (no trace)
to 10. Logging with high value of trace depth may affect performance.

### Graceful Shutdown

Package has a default flush timer of 100ms (configurable). If the program exits before it ticks, some logs may be lost.
To ensure logs are written, either add twice the flush timer wait, or use Shutdown() method.

```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond) // force shutdown after 0.5 second
defer cancel()

if err := logger.Shutdown(ctx); err != nil {
// Handle error
}
```

## Interfaces

The logger provides two sets of interfaces for different use cases:

### Full-featured logging with context support and structured key-value pairs:

```go
// with context
Init(ctx context.Context, cfg ...*LoggerConfig) error
Debug(ctx context.Context, args ...any)
Info(ctx context.Context, args ...any)
Warn(ctx context.Context, args ...any)
Error(ctx context.Context, args ...any)
DebugTrace(ctx context.Context, depth int, args ...any)
InfoTrace(ctx context.Context, depth int, args ...any)
WarnTrace(ctx context.Context, depth int, args ...any)
ErrorTrace(ctx context.Context, depth int, args ...any)
Shutdown(ctx context.Context) error
EnsureInitialized() bool
```

### Quick logging without context, auto-initializes if needed:

```go
// without context and initialization/config (default config is used in auto-initialization)
Config(args ...string)
Debug(args ...any)
Info(args ...any)
Warn(args ...any)
Error(args ...any)
Log(args ...any)
Message(args ...any)
DebugTrace(depth int, args ...any)
InfoTrace(depth int, args ...any)
WarnTrace(depth int, args ...any)
ErrorTrace(depth int, args ...any)
LogTrace(depth int, args ...any)
Shutdown()
```

## Implementation Details

- Uses atomic operations for counters and state management
- Single writer goroutine prevents disk contention
- Non-blocking channel handles logging bursts
- Efficient log rotation with unique timestamps
- Minimal lock contention using sync/atomic
- Automatic recovery of dropped logs on next successful write
- Context-aware goroutine operation and clean shutdown
- Graceful shutdown with 2x flush timer wait period for in-flight operations
- Silent log dropping on channel closure or disabled logger state
- Retention based on logs with same prefix having modified date/time within the Retention period

## License

BSD-3