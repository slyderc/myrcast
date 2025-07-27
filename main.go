package main

import (
	"errors"
	"flag"
	"path/filepath"

	"myrcast/config"
	"myrcast/internal/logger"
)

func main() {
	// Define command-line flags
	configPath := flag.String("config", getDefaultConfigPath(), "Path to TOML configuration file")
	logLevel := flag.String("log-level", "info", "Logging level (debug, info, warn, error)")
	logFile := flag.String("log-file", "", "Log output file (default: stdout)")
	generateConfig := flag.Bool("generate-config", false, "Generate a sample configuration file and exit")
	flag.Parse()

	// Handle config generation
	if *generateConfig {
		if err := config.GenerateSampleConfig(*configPath); err != nil {
			logger.Fatal("Failed to generate sample config: %v", err)
		}
		logger.Info("Sample configuration file created at: %s", *configPath)
		logger.Info("Please edit the file to add your API keys and customize settings")
		return
	}

	// Configure logging
	level, err := logger.ParseLevel(*logLevel)
	if err != nil {
		logger.Warn("Invalid log level: %s, using default (info)", *logLevel)
		level = logger.InfoLevel
	}
	logger.SetLevel(level)

	// Configure log output
	if *logFile != "" {
		if err := logger.SetOutput(*logFile); err != nil {
			logger.Error("Failed to set log file: %v", err)
		}
	}

	// Application startup
	logger.Info("Myrcast - Weather Report Generator")
	logger.Debug("Starting with config: %s", *configPath)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		var configNotFound *config.ConfigNotFoundError
		if errors.As(err, &configNotFound) {
			logger.Fatal("%v", err)
		} else {
			logger.Fatal("Failed to load configuration: %v", err)
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Configuration validation failed: %v", err)
	}

	// AIDEV-TODO: Use cfg to initialize application components
	logger.Info("Configuration loaded and validated from: %s", *configPath)
	logger.Debug("Weather location: %.4f, %.4f", cfg.Weather.Latitude, cfg.Weather.Longitude)
	logger.Debug("Units: %s", cfg.Weather.Units)
}

// getDefaultConfigPath returns a cross-platform default config path
func getDefaultConfigPath() string {
	// Try to use config.toml in the current directory
	return filepath.Clean("config.toml")
}
