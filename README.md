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
- JSON structured logging format
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
	// config is optional, values not set use default
	cfg := &logger.Config{
		Level:          logger.LevelInfo,
		Name:           "myapp",
		Directory:      "/var/log/myapp",
		BufferSize:     1000,
		MaxSizeMB:      100,  // Rotate files at 100MB
		MaxTotalSizeMB: 1000, // Keep total logs under 1GB
		MinDiskFreeMB:  500,  // Require 500MB free space
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

| Option         | Description                                      | Default   |
|----------------|--------------------------------------------------|-----------|
| Level          | Minimum log level to record                      | LevelInfo |
| Name           | Base name for log files                          | log       |
| Directory      | Directory to store log files                     | .         |
| BufferSize     | Channel buffer size for burst handling           | 1024      |
| MaxSizeMB      | Maximum size of each log file before rotation    | 10        |
| MaxTotalSizeMB | Maximum total size of log directory (0 disables) | 50        |
| MinDiskFreeMB  | Minimum required free disk space (0 disables)    | 100       |
| FlushTimer     | Time in milliseconds to force writing to disk    | 100       |

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

with context (requires initialization):

```go
logger.Init(ctx, cfg)
logger.Debug(ctx, "Debug message", "key1", "value1")
logger.Info(ctx, "Info message", "user", userID)
logger.Warn(ctx, "Warning message", "latency_ms", 150)
logger.Error(ctx, "Error message", "error", err.Error())
err := logger.Shutdown(ctx)
```

simplified, doesn't need initialization and shutdown (uses default config):

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
Init(ctx context.Context, cfg ...*Config) error
Debug(ctx context.Context, msg string, args ...any)
Info(ctx context.Context, msg string, args ...any)
Warn(ctx context.Context, msg string, args ...any)
Error(ctx context.Context, msg string, args ...any)
Shutdown(ctx context.Context) error
```

### Quick logging without context, auto-initializes if needed:

```go
// simplified
D(msg any, args ...any)
I(msg any, args ...any)
W(msg any, args ...any)
E(msg any, args ...any)
Shutdown() error
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
- Context-aware operations for clean shutdown

## License

MIT
