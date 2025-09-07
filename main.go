package main

import (
	"log"
	"log/slog"
	"os"

	Md "github.com/maroda/monteverdi/display"
	Ms "github.com/maroda/monteverdi/server"
)

// Set up default logging behavior
// Validate user is not root
func init() {
	file, err := os.OpenFile("monteverdi.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Print("Failed to open log file")
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	User := Ms.FillEnvVar("USER")
	if User == "root" {
		slog.Error("User cannot be root")
		os.Exit(1)
	}

	slog.Info("INITIALIZED Monteverdi", slog.String("user", User))
}

func main() {
	slog.Info("STARTING Monteverdi")

	// config filename version
	// TODO: make this an env var
	localJSON := "config.json"

	// Create config
	config, err := Ms.LoadConfigFileName(localJSON)
	if err != nil {
		slog.Error("Error loading config.json", slog.Any("Error", err))
		panic("Error loading config.json")
	}

	// Start Monteverdi
	err = Md.StartHarmonyViewWithConfig(config)
	if err != nil {
		slog.Error("Error starting harmony view", slog.Any("Error", err))
		panic("Error starting harmony view")
	}
}
