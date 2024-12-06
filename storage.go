package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	diskFullLogged atomic.Bool
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
		if !info.IsDir() && filepath.Ext(entry.Name()) == ".log" {
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
		if filepath.Ext(entry.Name()) != ".log" {
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