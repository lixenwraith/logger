// goroutine: the program demonstrates logger usage in goroutines with context timeout
package main

import (
	"context"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	ctx := context.Background()
	cfg := &logger.LoggerConfig{
		Level:     logger.LevelDebug,
		Directory: "./logs",
	}

	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	for i := 0; i < 5; i++ {
		go func(id int) {

			opCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			// Simulate work
			logger.Info(opCtx, "Starting operation", "id", id)
			time.Sleep(150 * time.Millisecond) // Deliberately exceed timeout
			logger.Info(opCtx, "Operation completed", "id", id)
		}(i)
	}

}