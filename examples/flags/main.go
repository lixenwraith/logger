package main

import (
	"github.com/LixenWraith/logger"
	"github.com/LixenWraith/logger/quick"
)

func main() {
	quick.Config("level=debug", "show_timestamp=true", "show_level=false")

	quick.Debug("normal debug message")
	quick.Log("log with timestamp only")
	quick.Message("raw message without headers")

	quick.Config("level=info", "show_timestamp=false", "format=json", "show_level=true")
	quick.Debug("debug in json") // won't be logged with level set to info
	quick.Info("info in json")
	quick.Log("log in json")
	quick.Message("message in json")

	logger.Shutdown()
}