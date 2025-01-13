// traced: the program demonstrates logger and logger/quick trace usage
package main

import (
	"context"
	"time"

	"github.com/LixenWraith/logger"
	"github.com/LixenWraith/logger/quick"
)

func main() {
	ctx := context.Background()
	cfg := &logger.LoggerConfig{
		Directory:  "./logs",
		TraceDepth: 8,
		Format:     "json",
	}

	if err := logger.Init(ctx, cfg); err != nil {
		panic(err)
	}
	defer logger.Shutdown(ctx)

	// Regular function call
	outerFunction()

	// Goroutine with anonymous function
	go func() {
		quick.Info("Log from anonymous goroutine")
		innerAnonymous()
	}()

	// Named function with anonymous inner
	mixedCalls()

	// Wait for goroutines to finish
	time.Sleep(time.Second)
}

func outerFunction() {
	quick.Info("Outer function log")
	middleFunction()
}

func middleFunction() {
	quick.Info("Middle function log")
	innerFunction()
}

func innerFunction() {
	quick.Info("Inner function log")
}

func innerAnonymous() {
	fn := func() {
		quick.Info("Nested anonymous log")
	}
	fn()
}

func mixedCalls() {
	quick.Info("Mixed calls start")

	func() {
		quick.Info("First anonymous")
		func() {
			quick.Info("Second anonymous")
			func() {
				quick.Info("Third anonymous")
			}()
		}()
	}()
}