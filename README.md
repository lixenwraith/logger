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
    cfg := &logger.Config{
        Level:          logger.LevelInfo,
        Name:           "myapp",
        Directory:      "/var/log/myapp",
        BufferSize:     1000,
        MaxSizeMB:      100,     // Rotate files at 100MB
        MaxTotalSizeMB: 1000,   // Keep total logs under 1GB
        MinDiskFreeMB:  500,    // Require 500MB free space
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

| Option | Description | Default |
|--------|-------------|---------|
| Level | Minimum log level to record | - |
| Name | Base name for log files | - |
| Directory | Directory to store log files | - |
| BufferSize | Channel buffer size for burst handling | 1000 |
| MaxSizeMB | Maximum size of each log file before rotation | - |
| MaxTotalSizeMB | Maximum total size of log directory (0 disables) | 0 |
| MinDiskFreeMB | Minimum required free disk space (0 disables) | 0 |

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

```go
logger.Debug(ctx, "Debug message", "key1", "value1")
logger.Info(ctx, "Info message", "user", userID)
logger.Warn(ctx, "Warning message", "latency_ms", 150)
logger.Error(ctx, "Error message", "error", err.Error())
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

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := logger.Shutdown(ctx); err != nil {
    // Handle error
}
```

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