package main

import (
	"flag"
	"fmt"
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
	configfile := flag.String("config", "config.json", "Path to configuration JSON")
	headless := flag.Bool("headless", false, "Container mode: no Terminal UI, logs sink to STDOUT")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Monteverdi - Seconda Practica Observability\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  MONTEVERDI_CONFIG_FILE\n")
		fmt.Fprintf(os.Stderr, "        Path to configuration file (default: config.json)\n")
		fmt.Fprintf(os.Stderr, "  MONTEVERDI_LOGLEVEL\n")
		fmt.Fprintf(os.Stderr, "        Log level: debug or info (default: info)\n")
		fmt.Fprintf(os.Stderr, "  MONTEVERDI_TUI_TSDB_VISUAL_WINDOW\n")
		fmt.Fprintf(os.Stderr, "        TUI display width in characters (default: 80)\n")
		fmt.Fprintf(os.Stderr, "  MONTEVERDI_PULSE_WINDOW_SECONDS\n")
		fmt.Fprintf(os.Stderr, "        Pulse lifecycle window in seconds (default: 3600)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -config=/path/to/config.json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -headless\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  MONTEVERDI_CONFIG_FILE=myconfig.json %s\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nRun with no options to start the terminal UI with webserver (port 8090).\n")
		fmt.Fprintf(os.Stderr, "There is a short warmup before pulses will appear in the web UI.\n")
		fmt.Fprintf(os.Stderr, "Logs sink to ./monteverdi.log unless in -headless mode.\n\n")
	}
	flag.Parse()

	// Determine config path: flag > env var > default
	configPath := *configfile
	if configPath == "" {
		configPath = Ms.FillEnvVar("MONTEVERDI_CONFIG_PATH")
		if configPath == "ENOENT" {
			configPath = "config.json"
		}
	}

	slog.Info("Configuration", slog.String("path", configPath))

	// Create config with filesystem
	// localfs := Ms.RealFS{}
	// config, err := Ms.LoadConfigFileNameWithFS(configPath, localfs)
	config, err := Ms.LoadConfigFileName(configPath)
	if err != nil {
		slog.Error("Error loading config.json", slog.Any("Error", err))
		panic("Error loading config.json")
	}

	// Start Monteverdi
	if *headless {
		// Run web-only version (no TUI)
		slog.Info("Using headless UI")
		err = Md.StartHarmonyViewWebOnly(config, configPath)
	} else {
		err = Md.StartHarmonyView(config, configPath)
	}
	if err != nil {
		slog.Error("Error starting harmony view", slog.Any("Error", err))
		panic("Error starting harmony view")
	}
}
