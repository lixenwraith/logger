package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type logRecord struct {
	LogCtx  context.Context
	Level   int
	Message string
	Args    []any
	Time    time.Time
}

var (
	logChannel    chan logRecord
	isInitialized atomic.Bool
	logLevel      atomic.Value // stores int
	logger        atomic.Value // stores *slog.Logger
	currentFile   atomic.Value // stores *os.File
	name          string
	directory     string
	maxSizeMB     int64
	currentSize   atomic.Int64
	bufferSize    atomic.Int64
	droppedLogs   atomic.Uint64
	processCtx    context.Context
	processCancel context.CancelFunc
	mu            sync.RWMutex
)

func generateLogFileName(ctx context.Context, baseName string, timestamp time.Time) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		baseTimestamp := timestamp.Format("060102_150405")
		// Always include first decimal place (tenth of a second)
		tenths := (timestamp.UnixNano() % 1e9) / 1e8
		filename := fmt.Sprintf("%s_%s.%d.log", baseName, baseTimestamp, tenths)
		fullPath := filepath.Join(directory, filename)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return filename, nil
		}

		// If file exists, try additional precision
		for precision := 2; precision <= 9; precision++ {
			subseconds := timestamp.UnixNano() % 1e9 / pow10(9-precision)
			subsecFormat := fmt.Sprintf("%%0%dd", precision)
			filename = fmt.Sprintf("%s_%s_%s.log",
				baseName,
				baseTimestamp,
				fmt.Sprintf(subsecFormat, subseconds))

			fullPath = filepath.Join(directory, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return filename, nil
			}
		}
		return "", fmt.Errorf("failed to generate unique log filename")
	}
}

func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

func createNewLogFile(ctx context.Context) (*os.File, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		filename, err := generateLogFileName(ctx, name, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to generate log filename: %w", err)
		}

		file, err := os.OpenFile(
			filepath.Join(directory, filename),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create log file: %w", err)
		}
		if err != nil {
			if file != nil {
				file.Close()
			}
			return nil, err
		}
		return file, nil
	}
}

func rotateLogFile(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		newFile, err := createNewLogFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create new log file: %w", err)
		}

		oldFile := currentFile.Load().(*os.File)
		if oldFile != nil {
			if err := oldFile.Close(); err != nil {
				newFile.Close()
				return fmt.Errorf("failed to close old log file: %w", err)
			}
		}

		currentFile.Store(newFile)
		currentSize.Store(0)

		logger.Store(slog.New(slog.NewJSONHandler(newFile, &slog.HandlerOptions{
			Level: slog.Level(logLevel.Load().(int)),
		})))

		return nil
	}
}

func processLogs() {
	for {
		select {
		case <-processCtx.Done():
			return
		case record, ok := <-logChannel:
			if !ok {
				return
			}

			select {
			case <-record.LogCtx.Done():
				continue
			default:
				log := logger.Load().(*slog.Logger)
				attrs := []slog.Attr{
					slog.Time("timestamp", record.Time),
					slog.Any("args", record.Args),
				}

				currentFileSize := currentSize.Load()
				estimatedSize := currentFileSize + 512

				if maxSizeMB > 0 && estimatedSize > maxSizeMB*1024*1024 {
					if err := rotateLogFile(record.LogCtx); err != nil {
						continue
					}
				}

				log.LogAttrs(record.LogCtx, slog.Level(record.Level), record.Message, attrs...)

				if fi, err := os.Stat(currentFile.Load().(*os.File).Name()); err == nil {
					currentSize.Store(fi.Size())
				}
			}
		}
	}
}

func initLogger(ctx context.Context, cfg *Config, level int) error {
	mu.Lock()
	defer mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		name = cfg.Name
		directory = cfg.Directory
		maxSizeMB = cfg.MaxSizeMB

		newBufferSize := int64(cfg.BufferSize)
		if newBufferSize < 1 {
			newBufferSize = 1000
		}

		if isInitialized.Load() {
			if processCancel != nil {
				processCancel()
			}

			logLevel.Store(level)

			if newBufferSize != bufferSize.Load() {
				oldChannel := logChannel
				logChannel = make(chan logRecord, newBufferSize)
				bufferSize.Store(newBufferSize)
				close(oldChannel)
			}
		}

		logFile, err := createNewLogFile(ctx)
		if err != nil {
			return fmt.Errorf("failed to create initial log file: %w", err)
		}

		currentFile.Store(logFile)
		logLevel.Store(level)
		bufferSize.Store(newBufferSize)

		logger.Store(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			Level: slog.Level(logLevel.Load().(int)),
		})))

		logChannel = make(chan logRecord, newBufferSize)

		processCtx, processCancel = context.WithCancel(ctx)
		go processLogs()

		isInitialized.Store(true)
		return nil
	}
}

func log(logCtx context.Context, level int, msg string, args ...any) {
	record := logRecord{
		LogCtx:  logCtx,
		Level:   level,
		Message: msg,
		Args:    args,
		Time:    time.Now(),
	}

	select {
	case logChannel <- record:
	case <-logCtx.Done():
		droppedLogs.Add(1)
	}
}

func shutdownLogger(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if !isInitialized.Load() {
		return nil
	}

	if processCancel != nil {
		processCancel()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		close(logChannel)

		if currentFile := currentFile.Load().(*os.File); currentFile != nil {
			if err := currentFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}

		isInitialized.Store(false)
		return nil
	}
}
