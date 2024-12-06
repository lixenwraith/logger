package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	cfg := &logger.LoggerConfig{
		Name:                   "test",
		Directory:              "./logs",
		RetentionPeriod:        0.000556, // 0.0005556 hour (~2 secs = 2/3600 hours)
		RetentionCheckInterval: 0.16,     // Check every 0.16 minute (+10 secs = 10/60 mins)
	}

	if err := logger.Init(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}

	// Write some logs
	for i := 0; i < 5; i++ {
		logger.I("test message", "count", i)
		if i == 2 {
			// Force rotate after 3rd message
			time.Sleep(time.Second)
			logger.Config(&logger.LoggerConfig{
				Name:                   "test",
				Directory:              "./logs",
				RetentionPeriod:        0.000556,
				RetentionCheckInterval: 1,
				MaxSizeMB:              0, // Force rotation
			})
		}
	}

	// Wait to see retention in action
	fmt.Println("Waiting 1 minutes")
	time.Sleep(1 * time.Minute)
	logger.Shutdown()
}