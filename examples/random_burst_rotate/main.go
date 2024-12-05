// Warning: this program uses around 50MB of disk space for dummy logs in "logs/" directory.
// high CPU and disk usage while writing around 2GB of logs to test rotation and drops.
// Tested with: 200 logs/~1MB memory buffer, 1000 parallel workers, local SSD.
// Result: 33,000 * ~10.3KB logs/sec or ~330MB/sec.

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/LixenWraith/logger"
)

const (
	totalBursts    = 200
	logsPerBurst   = 1000
	maxMessageSize = 20000
	numWorkers     = 1000
)

var levels = []int64{
	logger.LevelDebug,
	logger.LevelInfo,
	logger.LevelWarn,
	logger.LevelError,
}

func generateRandomMessage(size int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	var sb strings.Builder
	sb.Grow(size)
	for i := 0; i < size; i++ {
		sb.WriteByte(chars[rand.Intn(len(chars))])
	}
	return sb.String()
}

func logBurst(ctx context.Context, burstID int) {
	for i := 0; i < logsPerBurst; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			level := levels[rand.Intn(len(levels))]
			msgSize := rand.Intn(maxMessageSize) + 100
			msg := generateRandomMessage(msgSize)

			args := []any{
				"msg", msg,
				"worker_id", burstID % numWorkers,
				"burst_id", burstID,
				"log_number", i,
				"random_value", rand.Int63(),
				"timestamp", time.Now().UnixNano(),
			}

			switch level {
			case logger.LevelDebug:
				logger.Debug(ctx, args...)
			case logger.LevelInfo:
				logger.Info(ctx, args...)
			case logger.LevelWarn:
				logger.Warn(ctx, args...)
			case logger.LevelError:
				logger.Error(ctx, args...)
			}

			time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
		}
	}
}

func worker(ctx context.Context, burstChan chan int, wg *sync.WaitGroup, completedBursts *atomic.Int64) {
	defer wg.Done()

	for burstID := range burstChan {
		select {
		case <-ctx.Done():
			return
		default:
			logBurst(ctx, burstID)
			completed := completedBursts.Add(1)
			fmt.Printf("\rProgress: %d/%d bursts completed", completed, totalBursts)
		}
	}
}

func main() {
	rand.NewSource(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	logsDir := filepath.Join(currentDir, "logs")

	cfg := &logger.Config{
		Level:          logger.LevelDebug,
		Name:           "testapp",
		Directory:      logsDir,
		BufferSize:     200,
		MaxSizeMB:      1,
		MaxTotalSizeMB: 50,
		MinDiskFreeMB:  100,
	}

	if err := logger.Init(ctx, cfg); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Logger initialized. Logs will be written to: %s\n", logsDir)
	fmt.Printf("Starting stress test with %d workers generating %d bursts of %d logs each\n",
		numWorkers, totalBursts, logsPerBurst)
	fmt.Println("Press Ctrl+C to stop")

	// Create buffered channel for work distribution
	burstChan := make(chan int, totalBursts)
	var wg sync.WaitGroup
	completedBursts := atomic.Int64{}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, burstChan, &wg, &completedBursts)
	}

	// Start time tracking
	startTime := time.Now()

	// Handle shutdown signal
	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal. Waiting for current bursts to complete...")
		cancel()
	}()

	// Distribute work to workers
	for i := 1; i <= totalBursts; i++ {
		select {
		case <-ctx.Done():
			break
		case burstChan <- i:
		}
	}
	close(burstChan)

	// Wait for all workers to complete
	wg.Wait()
	duration := time.Since(startTime)

	fmt.Printf("\nCompleted %d bursts in %v\n", completedBursts.Load(), duration)
	fmt.Println("Shutting down logger...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := logger.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Error during logger shutdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Logger shutdown complete. Program finished.")
}
