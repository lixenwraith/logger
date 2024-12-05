package main

import (
	"context"
	"time"

	"github.com/LixenWraith/logger"
)

func main() {
	ctx := context.Background()
	cfg := &logger.Config{
		Directory:  "./logs",
		TraceDepth: 8,
	}

	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	// Regular function call
	outerFunction()

	// Goroutine with anonymous function
	go func() {
		logger.I("Log from anonymous goroutine")
		innerAnonymous()
	}()

	// Named function with anonymous inner
	mixedCalls()

	// Wait for goroutines to finish
	time.Sleep(time.Second)
}

func outerFunction() {
	logger.I("Outer function log")
	middleFunction()
}

func middleFunction() {
	logger.I("Middle function log")
	innerFunction()
}

func innerFunction() {
	logger.I("Inner function log")
}

func innerAnonymous() {
	fn := func() {
		logger.I("Nested anonymous log")
	}
	fn()
}

func mixedCalls() {
	logger.I("Mixed calls start")

	func() {
		logger.I("First anonymous")
		func() {
			logger.I("Second anonymous")
			func() {
				logger.I("Third anonymous")
			}()
		}()
	}()
}
