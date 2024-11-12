# Logger

A buffered, rotating logger package for Go that wraps slog with additional features for production use.

## Features

- Simple global logger interface
- Buffered logging with configurable capacity
- Automatic log file rotation based on size
- Unique timestamp-based log file naming
- Thread-safe operations
- Context-aware logging
- Multiple log levels (Debug, Info, Warn, Error)
- Log level write filtering
- Graceful shutdown support
- Runtime reconfiguration capability

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
        Level:      logger.LevelInfo,
        Name:       "myapp",
        Directory:  "/var/log/myapp",
        BufferSize: 1000,
        MaxSizeMB:  100,
    }

    ctx := context.Background()
    if err := logger.Init(ctx, cfg); err != nil {
        panic(err)
    }
    defer logger.Shutdown(ctx)

    // Use logger anywhere in your code
    logger.Info(ctx, "Application started", "version", "1.0.0")
}
```

## Configuration

The `Config` struct supports the following options:

- `Level`: Log level (LevelDebug, LevelInfo, LevelWarn, LevelError)
- `Name`: Base name for log files
- `Directory`: Directory to store log files
- `BufferSize`: Channel buffer size for handling burst loads
- `MaxSizeMB`: Maximum size of each log file before rotation

## Usage

### Logging

```go
logger.Debug(ctx, "Debug message", "key1", "value1")
logger.Info(ctx, "Info message", "key1", "value1")
logger.Warn(ctx, "Warning message", "key1", "value1")
logger.Error(ctx, "Error message", "key1", "value1")
```

### Runtime Reconfiguration

The logger can be reinitialized with new configuration.
There is a risk of some log loss in high traffic situaiton when buffer size is changed (to be fixed).

```go
newCfg := &logger.Config{
    Level:      logger.LevelDebug,
    Name:       "myapp",
    Directory:  "/new/log/path",
    BufferSize: 2000,
    MaxSizeMB:  200,
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

## Performance

The logger is designed to handle high-volume logging with:
- Buffered channel for burst handling
- Single writer goroutine to prevent contention
- Efficient file rotation
- Minimal lock contention

## Best Practices

1. Always provide a context to logging calls
2. Configure appropriate buffer size for your load
3. Set reasonable file size limits for rotation
4. Implement proper shutdown in your application
5. Monitor dropped logs in high-load scenarios

## License

BSD-2 - see LICENSE
