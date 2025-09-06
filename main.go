package main

import (
	"log/slog"

	Md "github.com/maroda/monteverdi/display"
	Ms "github.com/maroda/monteverdi/server"
)

func init() {
	User := Ms.FillEnvVar("USER")
	slog.Info("Monteverdi initializing for ... ", User)
}

func main() {
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
