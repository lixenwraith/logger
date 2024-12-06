# Logger

A buffered rotating logger for Go that wraps slog with advanced disk management and monitoring capabilities.

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
- Efficient JSON structured logging with zero allocations
- Function call trace support with configurable depth
- Graceful shutdown with context support
- Runtime reconfiguration
- Disk full protection with logging pause

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
	// config is optional, unconfigured parameters use default values
	cfg := &logger.Config{
		Level:          logger.LevelInfo,
		Name:           "myapp",
		Directory:      "/var/log/myapp",
		BufferSize:     1000,
		MaxSizeMB:      100,  // Rotate files at 100MB
		MaxTotalSizeMB: 1000, // Keep total logs under 1GB
		MinDiskFreeMB:  500,  // Require 500MB free space
        TraceDepth:     2,    // Include 2 levels of function calls in trace
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

The `Config` struct provides the following options:

| Option         | Description                                           | Default   |
|----------------|-------------------------------------------------------|-----------|
| Level          | Minimum log level to record                           | LevelInfo |
| Name           | Base name for log files                               | log       |
| Directory      | Directory to store log files                          | .         |
| BufferSize     | Channel buffer size for burst handling                | 1024      |
| MaxSizeMB      | Maximum size of each log file before rotation         | 10        |
| MaxTotalSizeMB | Maximum total size of log directory (0 disables)      | 50        |
| MinDiskFreeMB  | Minimum required free disk space (0 disables)         | 100       |
| FlushTimer     | Time in milliseconds to force writing to disk         | 100       |
| TraceDepth     | Number of function calls to include in trace (max 10) | 0         |

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
logger.I("Starting app")
logger.E(err, "operation", "db_connect", "retry", 3)
logger.W(customError{}, "component", "cache")
logger.D(response, "request_id", reqID)
_ := logger.Shutdown() // to ensure logs are written if the program finishes faster than flush timer
```

### Runtime Reconfiguration

The logger supports live reconfiguration while preserving existing logs:

```go
newCfg := &logger.Config{
Level:          logger.LevelDebug,
Name:           "myapp",
Directory:      "/new/log/path",
BufferSize:     2000,
MaxSizeMB:      200,
MaxTotalSizeMB: 2000,
MinDiskFreeMB:  1000,
}

if err := logger.Init(ctx, newCfg); err != nil {
// Handle error
}
```

### Function Call Tracing

The logger supports automatic function call tracing with configurable depth:

```go
cfg := &logger.Config{
TraceDepth: 3,  // Capture up to 3 levels of function calls
}
```

When enabled, each log entry will include the function call chain that led to the logging call. This helps with debugging and understanding the code flow. The trace depth can be set between 0 (disabled) and 10 levels.
Example output with TraceDepth=2:

```json
{
  "time": "2024-03-21T15:04:05.123456789Z",
  "level": "info",
  "fields": [
    **"main.processOrder -> main.validateInput"**,
    "Order validated",
    "order_id",
    "12345"
  ]
}
```
### Temporary Function Call Tracing

While the logger configuration supports persistent function call tracing, you can also enable tracing for specific log entries using trace variants of logging functions:

```go
// Context-aware logging with trace
logger.InfoTrace(3, ctx, "Processing order", "id", orderId)  // Shows 3 levels of function calls
logger.DebugTrace(2, ctx, "Cache miss", "key", cacheKey)     // Shows 2 levels
logger.WarnTrace(4, ctx, "Retry attempt", "count", retries)  // Shows 4 levels
logger.ErrorTrace(5, ctx, "Operation failed", "error", err)  // Shows 5 levels

// Simplified logging with trace
logger.IT(3, "Worker started", "pool", poolID)  // Info with 3 levels
logger.DT(2, "Request received")                // Debug with 2 levels
logger.WT(4, "Connection timeout")              // Warning with 4 levels
logger.ET(5, err, "Database error")            // Error with 5 levels
```
These functions temporarily override the configured TraceDepth for a single log entry. This is useful for debugging specific code paths without enabling tracing for all logs:

```go
func processOrder(orderID string) {
    // Normal log without trace
    logger.Info(ctx, "Processing started", "order", orderID)
    
    if err := validate(orderID); err != nil {
        // Log error with function call trace
        logger.ErrorTrace(3, ctx, "Validation failed", "error", err)
        return
    }
    
    // Back to normal logging
    logger.Info(ctx, "Processing completed", "order", orderID)
}
```
The trace depth parameter works the same way as the TraceDepth configuration option, accepting values from 0 (no trace) to 10.

### Graceful Shutdown

Package has a default flush timer of 100ms (configurable). If the program exits before it ticks there may be no logs
written.
To ensure logs are written either add twice the flush timer wait, or use Shutdown() method.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
Shutdown(ctx context.Context) error
```

### Quick logging without context, auto-initializes if needed:

```go
// simplified
// optional: Config(cfg *LoggerConfig) error
D(args ...any)
I(args ...any)
W(args ...any)
E(args ...any)
// optional: Shutdown() error
```

The msg parameter accepts various types:

- string: used directly
- error: error message is extracted
- fmt.Stringer: String() method is called
- other types: formatted using %+v

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

## License

MIT