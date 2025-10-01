package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	Md "github.com/maroda/monteverdi/display"
	Ms "github.com/maroda/monteverdi/server"
)

// Set up default logging behavior
// Validate user is not root
func init() {
	var logger *slog.Logger
	var loglevel slog.Level

	mode := Ms.FillEnvVar("MONTEVERDI_LOGLEVEL")
	if mode == "debug" {
		loglevel = slog.LevelDebug
	} else {
		loglevel = slog.LevelInfo
	}

	// Check if STDOUT is a terminal first
	fileInfo, _ := os.Stdout.Stat()
	isTTY := (fileInfo.Mode() & os.ModeCharDevice) != 0

	if isTTY {
		// Terminal mode: log to file
		file, err := os.OpenFile("monteverdi.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Print("Failed to open log file")
			os.Exit(1)
		}

		logger = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level:     loglevel,
			AddSource: true,
		}))
	} else {
		// Non-TTY, probably a container, so log to STDOUT
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     loglevel,
			AddSource: false,
		}))
	}

	slog.SetDefault(logger)

	User := Ms.FillEnvVar("USER")
	if User == "root" && isTTY {
		slog.Error("User cannot be root")
		os.Exit(1)
	}

	slog.Info("INITIALIZED Monteverdi", slog.String("user", User))
}

func main() {
	slog.Info("STARTING Monteverdi")

	// check if headless for container use
	headless := flag.Bool("headless", false, "Run with no Terminal UI for containers")
	flag.Parse()

	// config filename version
	// TODO: make this an env var
	localJSON := "config.json"

	// Create config with filesystem
	localfs := Ms.RealFS{}
	config, err := Ms.LoadConfigFileNameWithFS(localJSON, localfs)
	if err != nil {
		slog.Error("Error loading config.json", slog.Any("Error", err))
		panic("Error loading config.json")
	}

	// Start Monteverdi
	if *headless {
		// Run web-only version (no TUI)
		slog.Info("Using headless UI")
		err = Md.StartWebNoTUI(config)
	} else {
		err = Md.StartHarmonyViewWithConfig(config)
	}
	if err != nil {
		slog.Error("Error starting harmony view", slog.Any("Error", err))
		panic("Error starting harmony view")
	}
}
