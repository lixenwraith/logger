package main

import (
	"context"
	"sync"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	ctx := context.Background()
	cfg := &logger.Config{
		Level:          logger.LevelDebug,
		Name:           "test",
		Directory:      "./logs",
		BufferSize:     1000,
		MaxSizeMB:      10,
		MaxTotalSizeMB: 100,
		MinDiskFreeMB:  1000,
	}

	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			opCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			// Simulate work
			logger.Info(opCtx, "Starting operation", "id", id)
			time.Sleep(200 * time.Millisecond) // Deliberately exceed timeout
			logger.Info(opCtx, "Operation completed", "id", id)
		}(i)
	}

	wg.Wait()
	time.Sleep(time.Second) // Ensure all logs are processed
}
