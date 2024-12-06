// spot_traced: program demonstrates switching between traced and untraced logs
package main

import (
	"context"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	cfg := &logger.LoggerConfig{
		Name:      "test",
		Directory: "logs",
		Level:     logger.LevelDebug,
	}

	if err := logger.Config(cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown()

	// Function chain to test trace
	doSomething()

	// Same chain with trace
	time.Sleep(100 * time.Millisecond)
	doSomethingWithTrace()
}

func doSomething() {
	logger.Info(context.Background(), "Starting process")
	processData()
}

func processData() {
	logger.Info(context.Background(), "Processing data")
	finalStep()
}

func finalStep() {
	logger.Info(context.Background(), "Final step")
}

func doSomethingWithTrace() {
	logger.InfoTrace(3, context.Background(), "Starting process")
	processDataWithTrace()
}

func processDataWithTrace() {
	logger.InfoTrace(3, context.Background(), "Processing data")
	finalStepWithTrace()
}

func finalStepWithTrace() {
	logger.InfoTrace(3, context.Background(), "Final step")
}