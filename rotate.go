package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File naming variables
var (
	name string
)

// generateLogFileName creates a unique log filename using timestamp with increasing precision.
// It ensures uniqueness by progressively adding more precise subsecond components.
func generateLogFileName(ctx context.Context, baseName string, timestamp time.Time) (string, error) {
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

// pow10 calculates powers of 10 for subsecond precision in log filenames.
// It's used by generateLogFileName to create unique timestamp-based names.
func pow10(n int) int64 {
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// createNewLogFile generates and opens a new log file with proper permissions.
// It ensures unique naming and proper file creation with append mode.
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
		return file, nil
	}
}

// rotateLogFile handles the log rotation process, creating new file and closing old one.
// It updates all necessary state while maintaining thread safety.
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

		return nil
	}
}