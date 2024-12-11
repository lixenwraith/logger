// quick: the program demonstrates logger/quick interface usage
package main

import (
	"fmt"
	"github.com/LixenWraith/logger/quick"
	"os"
)

type customError struct {
	code    int
	message string
}

func (e customError) Error() string {
	return fmt.Sprintf("error %d: %s", e.code, e.message)
}

func main() {
	// Test automatic initialization
	quick.Info("Starting logger test")

	// Test different message types
	quick.Debug("Debug message", "component", "test")
	quick.Warn(fmt.Errorf("warning condition"), "severity", "medium")
	quick.Error(customError{500, "test error"}, "attempt", 1)

	// Test structured logging
	quick.Info("Operation complete",
		"duration_ms", 150,
		"success", true,
		"records", 42)

	// Wait for ticker to write
	quick.Shutdown()
	// Let the finalizer handle cleanup
	os.Exit(0)
}