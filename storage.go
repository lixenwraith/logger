package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Disk management and file state vars
var (
	directory   string
	currentSize atomic.Int64
	currentFile atomic.Value // stores *os.File

	maxSizeMB      int64
	maxTotalSizeMB int64
	minDiskFreeMB  int64

	diskFullLogged   atomic.Bool
	earliestFileTime atomic.Value // stores time.Time
	retentionPeriod  time.Duration
	retentionCheck   time.Duration
)

// getDiskStats retrieves filesystem statistics for the log directory.
// It returns available and total space in bytes.
func getDiskFreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks * block size
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// getLogDirSize calculates total size of all log files in the directory.
// It only counts files with .log extension.
func getLogDirSize(dir string) (int64, error) {
	var size int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.IsDir() && filepath.Ext(entry.Name()) == "."+extension {
			size += info.Size()
		}
	}
	return size, nil
}

// cleanOldLogs removes oldest log files to free up required disk space.
// It sorts files by modification time and removes them until enough space is freed.
func cleanOldLogs(ctx context.Context, required int64) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	// Build list of log files with their metadata
	type logFile struct {
		name    string
		modTime time.Time
		size    int64
	}

	var logs []logFile
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != "."+extension {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if f := currentFile.Load().(*os.File); f != nil &&
			entry.Name() == filepath.Base(f.Name()) {
			continue
		}
		logs = append(logs, logFile{
			name:    entry.Name(),
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}
	if len(logs) == 0 {
		return fmt.Errorf("no logs available to delete, needed %d bytes", required)
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].modTime.Before(logs[j].modTime)
	})

	var deleted int64
	for _, log := range logs {
		if deleted >= required {
			break
		}
		if err := os.Remove(filepath.Join(directory, log.name)); err != nil {
			continue
		}
		deleted += log.size
	}
	if deleted < required {
		return fmt.Errorf("could not free enough space: freed %d bytes, needed %d bytes", deleted, required)
	}
	return nil
}

// checkDiskSpace ensures sufficient disk space is available for logging.
// It manages disk space by cleaning up old logs and pausing logging if necessary.
func checkDiskSpace(ctx context.Context) error {
	// Skip check if disk management not configured
	if maxTotalSizeMB == 0 && minDiskFreeMB == 0 {
		return nil
	}

	// Check current disk space and directory size
	free, err := getDiskFreeSpace(directory)
	if err != nil {
		return err
	}

	dirSize, err := getLogDirSize(directory)
	if err != nil {
		return err
	}

	minFree := minDiskFreeMB * 1024 * 1024
	maxTotal := maxTotalSizeMB * 1024 * 1024

	if free < minFree || (maxTotal > 0 && dirSize > maxTotal) {
		required := int64(0)
		if free < minFree {
			required = minFree - free
		}
		if maxTotal > 0 && dirSize > maxTotal {
			exceeded := dirSize - maxTotal
			if exceeded > required {
				required = exceeded
			}
		}

		if err := cleanOldLogs(ctx, required); err != nil {
			if !diskFullLogged.Load() {
				diskFullLogged.Store(true)
				return fmt.Errorf("disk full")
			}
		}
	}

	diskFullLogged.Store(false)
	return nil
}

// updateEarliestFileTime scans the log directory and updates the atomic storage
// with the modification time of the oldest log file found.
func updateEarliestFileTime() {
	entries, err := os.ReadDir(directory)
	if err != nil {
		earliestFileTime.Store(time.Time{}) // Clear on error
		return
	}

	var earliest time.Time
	currentLogFile := ""
	if f := currentFile.Load().(*os.File); f != nil {
		currentLogFile = filepath.Base(f.Name())
	}

	// Format: <name>_<timestamp>.log or <name>_<timestamp>.<subsec>.log
	prefix := name + "_"

	for _, entry := range entries {
		// Skip nil entries
		if entry == nil {
			continue
		}

		// Get name once to avoid multiple calls
		fname := entry.Name()
		if fname == "" {
			continue
		}

		// Skip if not a log file or doesn't match the instance prefix
		if !strings.HasPrefix(fname, prefix) || filepath.Ext(fname) != "."+extension {
			continue
		}

		// Skip current log file
		if fname == currentLogFile {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if earliest.IsZero() || info.ModTime().Before(earliest) {
			earliest = info.ModTime()
		}
	}

	// Store even if zero - indicates no eligible files
	earliestFileTime.Store(earliest)
}

// cleanExpiredLogs removes any log file that matches the oldest known modification
// time in the directory. It skips the currently active log file and respects
// context cancellation.
func cleanExpiredLogs(ctx context.Context, oldest time.Time) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			if filepath.Ext(entry.Name()) != "."+extension {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Equal(oldest) {
				if f := currentFile.Load().(*os.File); f != nil &&
					entry.Name() == filepath.Base(f.Name()) {
					continue
				}
				if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}