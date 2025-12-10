package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/jhalter/mobius-hotline-client/internal"
	"github.com/muesli/termenv"
	"gopkg.in/yaml.v3"
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

	// Create config file with defaults if it doesn't exist
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		if err := createDefaultConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not create config file: %v\n", err)
			os.Exit(1)
		}
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

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".mobius-client-config.yaml")
}

func createDefaultConfig(path string) error {
	data, err := yaml.Marshal(internal.DefaultSettings)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
