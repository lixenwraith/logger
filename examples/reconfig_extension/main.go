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
		Format:    "json",
		Extension: "",
	}

	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	logger.Info(ctx, "Starting test", "ext", "log1")
	time.Sleep(200 * time.Millisecond)

	newCfg := &logger.LoggerConfig{
		Level:     logger.LevelDebug,
		Directory: "./logs",
		Format:    "json",
		Extension: "app",
	}

	if err := logger.Init(ctx, newCfg); err != nil {
		panic(err)
	}

	logger.Info(ctx, "After extension change", "ext", "log2")
	logger.Shutdown(ctx)
}