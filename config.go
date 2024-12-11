package logger

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Package level variables maintaining logger state and configuration.
// Thread-safety is ensured through atomic operations and mutex locks.
var (
	isInitialized atomic.Bool
	initMu        sync.Mutex

	loggerDisabled atomic.Bool

	logLevel atomic.Value // stores int64
	mu       sync.RWMutex
)

// LoggerConfig defines the logger configuration parameters.
// All fields can be configured via JSON or TOML configuration files.
type LoggerConfig struct {
	Level                  int64   `json:"level" toml:"level"`                                       // LevelDebug, LevelInfo, LevelWarn, LevelError
	Name                   string  `json:"name" toml:"name"`                                         // Base name for log files
	Directory              string  `json:"directory" toml:"directory"`                               // Directory to store log files
	Format                 string  `json:"format" toml:"format"`                                     // Serialized output file type: txt, json
	BufferSize             int64   `json:"buffer_size" toml:"buffer_size"`                           // Channel buffer size
	MaxSizeMB              int64   `json:"max_size_mb" toml:"max_size_mb"`                           // Max size of each log file in MB
	MaxTotalSizeMB         int64   `json:"max_total_size_mb" toml:"max_total_size_mb"`               // Max total size of the log folder in MB to trigger old log deletion/pause logging
	MinDiskFreeMB          int64   `json:"min_disk_free_mb" toml:"min_disk_free_mb"`                 // Min available free space in MB to trigger old log deletion/pause logging
	FlushTimer             int64   `json:"flush_timer" toml:"flush_timer"`                           // Periodically forces writing logs to the disk to avoid missing logs on program shutdown
	TraceDepth             int64   `json:"trace_depth" toml:"trace_depth"`                           // 0-10, 0 disables tracing
	RetentionPeriod        float64 `json:"retention_period" toml:"retention_period"`                 // RetentionPeriod defines how long to keep log files in hours. Zero disables retention.
	RetentionCheckInterval float64 `json:"retention_check_interval" toml:"retention_check_interval"` // RetentionCheckInterval defines how often to check for expired logs in minutes if retention is enabled.
}

// configLogger initializes the logger with the provided configuration.
// It validates the configuration and sets up the logging infrastructure including file management and buffering.
func configLogger(ctx context.Context, cfg ...*LoggerConfig) error {
	// defaultConfig values are used if value is not provided by the user
	defaultConfig := &LoggerConfig{
		Level:                  LevelInfo,
		Name:                   "log",
		Directory:              "./logs",
		Format:                 "txt",
		BufferSize:             1024,
		MaxSizeMB:              10,
		MaxTotalSizeMB:         50,
		MinDiskFreeMB:          100,
		FlushTimer:             100,
		TraceDepth:             0,
		RetentionPeriod:        0.0,
		RetentionCheckInterval: 60.0,
	}

	if len(cfg) == 0 {
		return initLogger(ctx, defaultConfig)
	} else {
		userConfig := cfg[0]
		mergedCfg := &LoggerConfig{
			Level:                  getConfigValue(defaultConfig.Level, userConfig.Level),
			Name:                   getConfigValue(defaultConfig.Name, userConfig.Name),
			Directory:              getConfigValue(defaultConfig.Directory, userConfig.Directory),
			Format:                 getConfigValue(defaultConfig.Format, userConfig.Format),
			BufferSize:             getConfigValue(defaultConfig.BufferSize, userConfig.BufferSize),
			MaxSizeMB:              getConfigValue(defaultConfig.MaxSizeMB, userConfig.MaxSizeMB),
			MaxTotalSizeMB:         getConfigValue(defaultConfig.MaxTotalSizeMB, userConfig.MaxTotalSizeMB),
			MinDiskFreeMB:          getConfigValue(defaultConfig.MinDiskFreeMB, userConfig.MinDiskFreeMB),
			FlushTimer:             getConfigValue(defaultConfig.FlushTimer, userConfig.FlushTimer),
			TraceDepth:             getConfigValue(defaultConfig.TraceDepth, userConfig.TraceDepth),
			RetentionPeriod:        getConfigValue(defaultConfig.RetentionPeriod, userConfig.RetentionPeriod),
			RetentionCheckInterval: getConfigValue(defaultConfig.RetentionCheckInterval, userConfig.RetentionCheckInterval),
		}
		return initLogger(ctx, mergedCfg)
	}
}

// initLogger configures and starts the logging infrastructure with the provided configuration.
// It handles initialization of files, channels, and background processing while ensuring thread safety.
func initLogger(ctx context.Context, cfg *LoggerConfig) error {
	mu.Lock()
	defer mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Setup directory and validate disk space configuration
		directory = cfg.Directory
		if directory == "" {
			directory = "."
		}
		if err := os.MkdirAll(directory, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		name = cfg.Name
		format = cfg.Format
		maxSizeMB = cfg.MaxSizeMB
		maxTotalSizeMB = cfg.MaxTotalSizeMB
		minDiskFreeMB = cfg.MinDiskFreeMB
		flushTimer = time.Duration(cfg.FlushTimer) * time.Millisecond
		retentionPeriod = time.Duration(cfg.RetentionPeriod * float64(time.Hour))
		retentionCheck = time.Duration(cfg.RetentionCheckInterval * float64(time.Minute))

		newBufferSize := cfg.BufferSize
		if newBufferSize < 1 {
			newBufferSize = 1000
		}

		if maxTotalSizeMB < 0 || minDiskFreeMB < 0 {
			return fmt.Errorf("invalid disk space configuration")
		}

		if cfg.TraceDepth < 0 || cfg.TraceDepth > 10 {
			return fmt.Errorf("invalid trace depth: must be between 0 and 10")
		}
		traceDepth = cfg.TraceDepth

		// Handle reconfiguration of existing logger
		if isInitialized.Load() {
			if processCancel != nil {
				processCancel()
			}

			logLevel.Store(cfg.Level)

			if newBufferSize != bufferSize.Load() {
				oldChannel := logChannel
				logChannel = make(chan logRecord, newBufferSize)
				bufferSize.Store(newBufferSize)
				close(oldChannel)
			}
		}

		// Initialize new log file and logger instance
		logFile, err := createNewLogFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create initial log file: %w", err)
		}

		currentFile.Store(logFile)
		logLevel.Store(cfg.Level)
		bufferSize.Store(newBufferSize)

		logChannel = make(chan logRecord, newBufferSize)

		processCtx, processCancel = context.WithCancel(ctx)
		go processLogs()

		isInitialized.Store(true)
		return nil
	}
}

// getConfigValue returns defaultVal if cfgVal equals the zero value for type T,
// otherwise returns cfgVal. Type T must satisfy the comparable constraint.
// This is commonly used for merging configuration values with their defaults.
func getConfigValue[T comparable](defaultVal, cfgVal T) T {
	var zero T
	if cfgVal == zero {
		return defaultVal
	}
	return cfgVal
}

// shutdownOnce ensures the logger shutdown routine executes exactly once,
// even if multiple shutdown paths are triggered simultaneously.
var shutdownOnce sync.Once

// generateLogFileName creates a unique log filename using timestamp with increasing precision.
// It ensures uniqueness by progressively adding more precise subsecond components.
func shutdownLogger(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if !isInitialized.Load() {
		return nil
	}

	timer := time.NewTimer(2 * flushTimer)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	loggerDisabled.Store(true)
	isInitialized.Store(false)

	if processCancel != nil {
		processCancel()
	}
	close(logChannel)

	// Final file operations
	if currentFile := currentFile.Load().(*os.File); currentFile != nil {
		syncDone := make(chan error, 1)
		go func() {
			syncDone <- currentFile.Sync()
		}()

		// Wait for sync or context cancellation
		select {
		case err := <-syncDone:
			if err != nil {
				return fmt.Errorf("failed to sync log file: %w", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}

		// Close file - this should be quick and not block
		if err := currentFile.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
	}

	return nil
}