package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	var wg sync.WaitGroup

	// Test concurrent logging
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Mix of different log types
			logger.I(fmt.Sprintf("Worker %d started", id))

			// Simulate work and logging
			for j := 0; j < 5; j++ {
				time.Sleep(100 * time.Millisecond)

				if j%2 == 0 {
					logger.W("Processing",
						"worker", id,
						"iteration", j,
						"status", "retry")
				} else {
					logger.I("Processed",
						"worker", id,
						"iteration", j,
						"status", "success")
				}
			}

			// Test error logging with custom type
			if id == 1 {
				err := customError{
					code:    503,
					message: "service unavailable",
				}
				logger.E(err,
					"worker", id,
					"fatal", true)
			}

			logger.I(fmt.Sprintf("Worker %d finished", id))
		}(i)
	}

	// Wait for goroutines
	wg.Wait()

	// Test large structured log
	logger.I("Test summary",
		"workers", 3,
		"iterations", 5,
		"errors", 1,
		"timestamp", time.Now().Unix(),
		"metadata", map[string]interface{}{
			"host":    "localhost",
			"pid":     os.Getpid(),
			"version": "1.0.0",
		})

	logger.Shutdown()
	time.Sleep(1 * time.Second)

	// Force immediate exit to test finalizer
	os.Exit(0)
}

type customError struct {
	code    int
	message string
}

func (e customError) Error() string {
	return fmt.Sprintf("error %d: %s", e.code, e.message)
}
