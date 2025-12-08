package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/jhalter/mobius-hotline-client/internal"
	"github.com/muesli/termenv"
)

// Values swapped in by go-releaser at build time
var (
	version = "dev"
)

var logLevels = map[string]log.Level{
	"debug": log.DebugLevel,
	"info":  log.InfoLevel,
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "Path to config file")
	logLevel := flag.String("log-level", "info", "Log level (debug, info)")

	flag.Parse()

	// Check if config file exists before proceeding
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config file not found: %s\n\n", *configPath)
		flag.Usage()
		os.Exit(1)
	}

	// init DebugBuffer
	db := &internal.DebugBuffer{}

	logHandler := log.New(db)

	// Force color output for logger.
	// By default, the charm logger package disables color for non-TTY.
	logHandler.SetColorProfile(termenv.TrueColor)
	logHandler.SetLevel(logLevels[*logLevel])

	logger := slog.New(logHandler)
	logger.Info("Started Mobius client", "Version", version)

	model := internal.NewModel(*configPath, logger, db)
	if err := model.Start(); err != nil {
		logger.Error("Application error", "err", err)
		os.Exit(1)
	}
}

func defaultConfigPath() (cfgPath string) {
	switch runtime.GOOS {
	case "windows":
		cfgPath = "mobius-client-config.yaml"
	case "darwin":
		if _, err := os.Stat("/usr/local/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/usr/local/etc/mobius-client-config.yaml"
		} else if _, err := os.Stat("/opt/homebrew/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/opt/homebrew/etc/mobius-client-config.yaml"
		} else {
			cfgPath = "mobius-client-config.yaml"
		}
	case "linux":
		cfgPath = "/usr/local/etc/mobius-client-config.yaml"
	default:
		fmt.Printf("unsupported OS")
		os.Exit(1)
	}

	return cfgPath
}
