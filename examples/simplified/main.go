package main

import (
	"fmt"
	"github.com/LixenWraith/logger"
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
	logger.I("Starting logger test")

	// Test different message types
	logger.D("Debug message", "component", "test")
	logger.W(fmt.Errorf("warning condition"), "severity", "medium")
	logger.E(customError{500, "test error"}, "attempt", 1)

	// Test structured logging
	logger.I("Operation complete",
		"duration_ms", 150,
		"success", true,
		"records", 42)

	// Wait for ticker to write
	logger.Shutdown()
	// Let the finalizer handle cleanup
	os.Exit(0)
}
