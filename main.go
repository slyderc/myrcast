package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myrcast/api"
	"myrcast/config"
	"myrcast/internal/logger"
)

const (
	Version = "1.0.0"
	AppName = "Myrcast"
)

// Exit codes for different scenarios
const (
	ExitSuccess         = 0 // Successful execution
	ExitGeneralError    = 1 // General errors
	ExitConfigError     = 2 // Configuration file errors
	ExitValidationError = 3 // Configuration validation errors
	ExitAPIError        = 4 // API call failures
	ExitFileSystemError = 5 // File system operation errors
	ExitNetworkError    = 6 // Network connectivity errors
)

func main() {
	startTime := time.Now()

	// Define command-line flags
	configPath := flag.String("config", getDefaultConfigPath(), "Path to TOML configuration file")
	logLevel := flag.String("log-level", "info", "Logging level (debug, info, warn, error)")
	logFile := flag.String("log-file", "", "Log output file (default: stdout)")
	generateConfig := flag.Bool("generate-config", false, "Generate a sample configuration file and exit")
	showVersion := flag.Bool("version", false, "Show version information and exit")
	showHelp := flag.Bool("help", false, "Show help information and exit")
	dryRun := flag.Bool("dry-run", false, "Validate configuration and show what would happen without executing")
	verbose := flag.Bool("verbose", false, "Enable verbose output (equivalent to --log-level=debug)")

	// Override default usage function
	flag.Usage = func() {
		showUsage()
	}

	flag.Parse()

	// Handle help flag
	if *showHelp {
		showUsage()
		os.Exit(0)
	}

	// Handle version flag
	if *showVersion {
		fmt.Printf("%s version %s\n", AppName, Version)
		os.Exit(0)
	}

	// Validate log level
	if *logLevel != "" {
		validLevels := []string{"debug", "info", "warn", "error"}
		if !contains(validLevels, strings.ToLower(*logLevel)) {
			fmt.Fprintf(os.Stderr, "Error: Invalid log level '%s'. Valid levels: %s\n",
				*logLevel, strings.Join(validLevels, ", "))
			os.Exit(ExitGeneralError)
		}
	}

	// Handle verbose flag
	if *verbose {
		*logLevel = "debug"
	}

	// Validate config path exists (unless generating config)
	if !*generateConfig && *configPath != "" {
		if err := validateConfigPath(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(ExitConfigError)
		}
	}

	flag.Parse()

	// Handle config generation
	if *generateConfig {
		if err := config.GenerateSampleConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to generate sample config: %v\n", err)
			os.Exit(ExitFileSystemError)
		}
		fmt.Printf("Sample configuration file created at: %s\n", *configPath)
		fmt.Printf("Please edit the file to add your API keys and customize settings\n")
		os.Exit(ExitSuccess)
	}

	// Configure enhanced logging before loading config
	// Use basic console logging for config loading phase
	tempLogConfig := logger.Config{
		Enabled:         false, // Console only during startup
		ConsoleOutput:   true,
		Level:           *logLevel,
		FilenamePattern: "myrcast-startup.log",
		Directory:       "logs",
		MaxFiles:        7,
		MaxSizeMB:       10,
	}

	// Override with log file if specified
	if *logFile != "" {
		tempLogConfig.Enabled = true
		tempLogConfig.Directory = filepath.Dir(*logFile)
		tempLogConfig.FilenamePattern = filepath.Base(*logFile)
	}

	if err := logger.Initialize(tempLogConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
		// Continue with fallback logging
	}

	// Application startup
	logger.Info("Myrcast - Weather Report Generator")
	logger.Debug("Starting with config: %s", *configPath)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		var configNotFound *config.ConfigNotFoundError
		if errors.As(err, &configNotFound) {
			logger.Error("%v", err)
			os.Exit(ExitConfigError)
		} else {
			logger.Error("Failed to load configuration: %v", err)
			os.Exit(ExitConfigError)
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
		os.Exit(ExitValidationError)
	}

	logger.Debug("Configuration loaded and validated from: %s", *configPath)

	// Reinitialize logging with configuration settings (unless overridden by command line)
	finalLogConfig := logger.Config{
		Enabled:         cfg.Logging.Enabled,
		Directory:       cfg.Logging.Directory,
		FilenamePattern: cfg.Logging.FilenamePattern,
		Level:           cfg.Logging.Level,
		MaxFiles:        cfg.Logging.MaxFiles,
		MaxSizeMB:       cfg.Logging.MaxSizeMB,
		ConsoleOutput:   cfg.Logging.ConsoleOutput,
	}

	// Override config settings with command line flags
	if *logLevel != "info" { // info is default, so only override if different
		finalLogConfig.Level = *logLevel
	}
	if *logFile != "" {
		finalLogConfig.Enabled = true
		finalLogConfig.Directory = filepath.Dir(*logFile)
		finalLogConfig.FilenamePattern = filepath.Base(*logFile)
	}

	// Reinitialize logger with final configuration
	if err := logger.Initialize(finalLogConfig); err != nil {
		logger.Warn("Failed to reinitialize logging with config settings: %v", err)
		// Continue with current logging setup
	} else {
		logger.Debug("Enhanced logging initialized from configuration")
	}

	logger.Debug("Weather location: %.4f, %.4f", cfg.Weather.Latitude, cfg.Weather.Longitude)
	logger.Debug("Units: %s", cfg.Weather.Units)

	// Handle dry-run mode
	if *dryRun {
		logger.Info("DRY RUN MODE - Showing what would happen without executing")
		logger.Info("Weather API: Would fetch weather for lat=%.4f, lon=%.4f using %s units",
			cfg.Weather.Latitude, cfg.Weather.Longitude, cfg.Weather.Units)
		logger.Info("Claude API: Would generate weather report using model %s", cfg.Claude.Model)
		logger.Info("ElevenLabs API: Would synthesize speech using voice %s", cfg.ElevenLabs.VoiceID)
		logger.Info("Output: Would save WAV file to %s", cfg.Output.ImportPath)
		logger.Info("All configuration checks passed - ready for production run")
		logger.Info("Remove --dry-run flag to execute actual weather report generation")
		return
	}

	// Run the main weather report generation workflow
	if err := runWeatherReportWorkflow(cfg); err != nil {
		logger.Error("Weather report generation failed: %v", err)

		// Log execution summary for failed run
		results := []string{
			fmt.Sprintf("Weather report generation failed: %v", err),
		}

		var exitCode int
		if isAPIError(err) {
			exitCode = ExitAPIError
		} else if isNetworkError(err) {
			exitCode = ExitNetworkError
		} else if isFileSystemError(err) {
			exitCode = ExitFileSystemError
		} else {
			exitCode = ExitGeneralError
		}

		logger.Get().LogExecutionSummary(startTime, *configPath, "weather-report", results, exitCode)
		os.Exit(exitCode)
	}

	logger.Info("Weather report generation completed successfully")

	// Log execution summary for successful run
	results := []string{
		"Weather report generation completed successfully",
		fmt.Sprintf("Weather location: %.4f, %.4f", cfg.Weather.Latitude, cfg.Weather.Longitude),
		fmt.Sprintf("Output directory: %s", cfg.Output.ImportPath),
	}
	logger.Get().LogExecutionSummary(startTime, *configPath, "weather-report", results, ExitSuccess)

	os.Exit(ExitSuccess)
}

// getDefaultConfigPath returns a cross-platform default config path
func getDefaultConfigPath() string {
	// Try to use config.toml in the current directory
	return filepath.Clean("config.toml")
}

// showUsage displays comprehensive help information
func showUsage() {
	fmt.Printf("%s - Weather Report Generator for Radio Broadcast\n\n", AppName)
	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [options]\n\n", strings.ToLower(AppName))

	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("  Generates AI-voiced weather reports for Myriad radio automation.\n")
	fmt.Printf("  Creates WAV files with weather information from OpenWeather API,\n")
	fmt.Printf("  processed through Anthropic Claude AI and ElevenLabs text-to-speech.\n\n")

	fmt.Printf("OPTIONS:\n")
	flag.PrintDefaults()

	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("  # Generate a weather report using default config\n")
	fmt.Printf("  %s\n\n", strings.ToLower(AppName))
	fmt.Printf("  # Use custom config file\n")
	fmt.Printf("  %s --config /path/to/custom.toml\n\n", strings.ToLower(AppName))
	fmt.Printf("  # Generate sample config file\n")
	fmt.Printf("  %s --generate-config --config example.toml\n\n", strings.ToLower(AppName))
	fmt.Printf("  # Run with verbose logging\n")
	fmt.Printf("  %s --verbose\n\n", strings.ToLower(AppName))
	fmt.Printf("  # Validate configuration without executing\n")
	fmt.Printf("  %s --dry-run\n\n", strings.ToLower(AppName))

	fmt.Printf("CONFIGURATION:\n")
	fmt.Printf("  Configuration file should contain API keys for:\n")
	fmt.Printf("  - OpenWeather API (weather data)\n")
	fmt.Printf("  - Anthropic Claude API (text generation)\n")
	fmt.Printf("  - ElevenLabs API (text-to-speech)\n\n")

	fmt.Printf("  Use --generate-config to create a sample configuration file.\n\n")

	fmt.Printf("OUTPUT:\n")
	fmt.Printf("  Generated WAV files are saved to the configured import directory\n")
	fmt.Printf("  for use with Myriad radio automation software.\n\n")

	fmt.Printf("VERSION:\n")
	fmt.Printf("  %s version %s\n\n", AppName, Version)
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// validateConfigPath validates that a config file path exists and is readable
// Includes Windows-specific path validation for UNC paths and long path names
func validateConfigPath(path string) error {
	// Clean the path for cross-platform compatibility
	cleanPath := filepath.Clean(path)

	// Windows-specific path validation
	if err := validateWindowsPath(cleanPath); err != nil {
		return err
	}

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("configuration file does not exist: %s", cleanPath)
		}
		return fmt.Errorf("cannot access configuration file: %s (%v)", cleanPath, err)
	}

	// Check if it's a regular file
	if info.IsDir() {
		return fmt.Errorf("configuration path is a directory, not a file: %s", cleanPath)
	}

	// Try to open the file to check readability
	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("cannot read configuration file: %s (%v)", cleanPath, err)
	}
	file.Close()

	return nil
}

// validateWindowsPath performs Windows-specific path validation
func validateWindowsPath(path string) error {
	// On Windows, handle UNC paths and long path names
	if filepath.VolumeName(path) != "" || strings.HasPrefix(path, `\\`) {
		// This is likely a Windows path with volume or UNC format

		// Check for invalid characters in Windows filenames
		invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
		baseName := filepath.Base(path)
		for _, char := range invalidChars {
			if strings.Contains(baseName, char) {
				return fmt.Errorf("configuration filename contains invalid character '%s': %s", char, baseName)
			}
		}

		// Check for reserved Windows filenames
		reservedNames := []string{
			"CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		}
		baseNameUpper := strings.ToUpper(strings.TrimSuffix(baseName, filepath.Ext(baseName)))
		for _, reserved := range reservedNames {
			if baseNameUpper == reserved {
				return fmt.Errorf("configuration filename uses reserved Windows name: %s", baseName)
			}
		}

		// Check for paths that are too long (Windows has 260 char limit for most APIs)
		if len(path) > 259 {
			return fmt.Errorf("configuration file path too long (%d chars, max 259): %s", len(path), path)
		}
	}

	return nil
}

// runWeatherReportWorkflow orchestrates the complete weather report generation process
func runWeatherReportWorkflow(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Debug("Starting weather report generation workflow")

	// Step 1: Initialize API clients
	logger.Debug("Initializing API clients...")

	// Initialize Weather client
	weatherClient := api.NewWeatherClientWithRateLimit(cfg.APIs.OpenWeather)
	logger.Debug("Weather client initialized")

	// Initialize Claude client
	claudeConfig := api.ClaudeConfig{
		APIKey:      cfg.APIs.Anthropic,
		Model:       cfg.Claude.Model,
		MaxTokens:   cfg.Claude.MaxTokens,
		Temperature: cfg.Claude.Temperature,
		MaxRetries:  cfg.Claude.MaxRetries,
		BaseDelay:   time.Duration(cfg.Claude.BaseDelayMs) * time.Millisecond,
		MaxDelay:    time.Duration(cfg.Claude.MaxDelayMs) * time.Millisecond,
		RateLimit:   cfg.Claude.RateLimit,
	}
	claudeClient, err := api.NewClaudeClient(claudeConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize Claude client: %w", err)
	}
	logger.Debug("Claude client initialized")

	// Initialize ElevenLabs client
	elevenLabsConfig := api.ElevenLabsConfig{
		APIKey:     cfg.APIs.ElevenLabs,
		VoiceID:    cfg.ElevenLabs.VoiceID,
		Model:      cfg.ElevenLabs.Model,
		Stability:  cfg.ElevenLabs.Stability,
		Similarity: cfg.ElevenLabs.Similarity,
		Style:      cfg.ElevenLabs.Style,
		Speed:      cfg.ElevenLabs.Speed,
		Format:     cfg.ElevenLabs.Format,
		MaxRetries: cfg.ElevenLabs.MaxRetries,
		BaseDelay:  time.Duration(cfg.ElevenLabs.BaseDelayMs) * time.Millisecond,
		MaxDelay:   time.Duration(cfg.ElevenLabs.MaxDelayMs) * time.Millisecond,
		RateLimit:  cfg.ElevenLabs.RateLimit,
	}
	elevenLabsClient, err := api.NewElevenLabsClient(elevenLabsConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize ElevenLabs client: %w", err)
	}
	logger.Debug("ElevenLabs client initialized")

	// Step 2: Initialize cache manager
	cacheManager := api.NewCacheManager(cfg.Cache.FilePath)
	logger.Debug("Cache manager initialized with file: %s", cfg.Cache.FilePath)

	// Step 3: Fetch weather data using One Call API (with caching)
	logger.Info("Fetching weather data using One Call API...")
	forecastParams := api.ForecastParams{
		Latitude:  cfg.Weather.Latitude,
		Longitude: cfg.Weather.Longitude,
		Units:     cfg.Weather.Units,
	}

	// Use the new One Call API with cache support
	todayWeather, oneCallData, err := weatherClient.GetTodayWeatherWithOneCallCache(ctx, forecastParams, cfg.Weather.Units, cacheManager)
	if err != nil {
		return fmt.Errorf("failed to fetch weather data: %w", err)
	}
	logger.Debug("Weather data fetched successfully for location: %.4f, %.4f",
		cfg.Weather.Latitude, cfg.Weather.Longitude)
	logger.Debug("Current conditions: %s, %.1f%s",
		todayWeather.CurrentConditions, todayWeather.CurrentTemp,
		api.GetUnitSuffix("temperature", cfg.Weather.Units))
	logger.Debug("Daily forecast: High=%.1f%s, Low=%.1f%s (from One Call API)",
		todayWeather.TempHigh, api.GetUnitSuffix("temperature", cfg.Weather.Units),
		todayWeather.TempLow, api.GetUnitSuffix("temperature", cfg.Weather.Units))

	// Step 4: Generate weather report script using Claude
	logger.Info("Generating weather report script...")

	// Convert One Call data to ForecastResponse format for backward compatibility with Claude
	// This allows the Claude client to work with both APIs
	forecast := convertOneCallToForecastResponse(oneCallData, todayWeather)

	reportRequest := api.WeatherReportRequest{
		PromptTemplate: cfg.Prompt.Template,
		WeatherData:    forecast,
		TodayData:      todayWeather, // Pass the real extracted data with correct daily min/max
		Location:       fmt.Sprintf("%.4f, %.4f", cfg.Weather.Latitude, cfg.Weather.Longitude),
		OutputPath:     cfg.Output.ImportPath,
	}

	reportResponse, err := claudeClient.GenerateWeatherReport(ctx, reportRequest)
	if err != nil {
		return fmt.Errorf("failed to generate weather report script: %w", err)
	}
	logger.Debug("Weather report script generated successfully (%d characters)", len(reportResponse.Script))

	// Step 5: Convert script to speech using ElevenLabs
	logger.Info("Converting script to speech...")

	// Ensure import directory exists
	if err := os.MkdirAll(cfg.Output.ImportPath, 0755); err != nil {
		return fmt.Errorf("failed to create import directory: %w", err)
	}

	speechRequest := api.TextToSpeechRequest{
		Text:      reportResponse.Script,
		OutputDir: cfg.Output.ImportPath, // Output directly to final location
		FileName:  cfg.Output.MediaID,
	}

	speechResponse, err := elevenLabsClient.GenerateTextToSpeech(ctx, speechRequest)
	if err != nil {
		return fmt.Errorf("failed to convert script to speech: %w", err)
	}
	logger.Debug("Speech generation completed successfully")
	logger.Debug("Audio file created: %s (%d ms)", speechResponse.AudioFilePath, speechResponse.DurationMs)

	// File is already saved to final location
	logger.Debug("Weather report saved successfully: %s", speechResponse.AudioFilePath)
	logger.Debug("Ready for import into Myriad radio automation")

	return nil
}

// Helper functions for error type checking
func isAPIError(err error) bool {
	return strings.Contains(err.Error(), "API") ||
		strings.Contains(err.Error(), "failed to generate") ||
		strings.Contains(err.Error(), "failed to convert")
}

func isNetworkError(err error) bool {
	return strings.Contains(err.Error(), "network") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection")
}

func isFileSystemError(err error) bool {
	return strings.Contains(err.Error(), "failed to create") ||
		strings.Contains(err.Error(), "failed to copy") ||
		strings.Contains(err.Error(), "directory") ||
		strings.Contains(err.Error(), "file")
}

// convertOneCallToForecastResponse converts One Call API data to ForecastResponse format
// for backward compatibility with existing Claude integration
func convertOneCallToForecastResponse(oneCall *api.OneCallResponse, todayData *api.TodayWeatherData) *api.ForecastResponse {
	if oneCall == nil {
		// Create minimal structure if no One Call data available (shouldn't happen normally)
		return &api.ForecastResponse{
			List: []api.ForecastItem{
				{
					Dt: time.Now().Unix(),
					Main: api.MainWeatherData{
						Temp:      todayData.CurrentTemp,
						FeelsLike: todayData.CurrentTemp,
						TempMin:   todayData.TempLow,
						TempMax:   todayData.TempHigh,
						Pressure:  1013.25,
						Humidity:  50,
					},
					Weather: []api.WeatherCondition{
						{
							Main:        todayData.CurrentConditions,
							Description: todayData.CurrentConditions,
						},
					},
					Wind: api.WindData{
						Speed: 10.0,
						Deg:   180,
					},
					Pop: todayData.RainChance,
				},
			},
			City: api.CityInfo{
				Name: todayData.Location,
			},
		}
	}

	// Convert One Call data to ForecastResponse format
	forecast := &api.ForecastResponse{
		Cod: "200",
		Cnt: 1,
		List: []api.ForecastItem{
			{
				Dt: oneCall.Current.Dt,
				Main: api.MainWeatherData{
					Temp:      oneCall.Current.Temp,
					FeelsLike: oneCall.Current.FeelsLike,
					TempMin:   todayData.TempLow,  // Use extracted daily low
					TempMax:   todayData.TempHigh, // Use extracted daily high
					Pressure:  float64(oneCall.Current.Pressure),
					Humidity:  oneCall.Current.Humidity,
				},
				Weather: oneCall.Current.Weather,
				Clouds:  api.CloudData{All: oneCall.Current.Clouds},
				Wind: api.WindData{
					Speed: oneCall.Current.WindSpeed,
					Deg:   float64(oneCall.Current.WindDeg),
					Gust:  oneCall.Current.WindGust,
				},
				Pop: todayData.RainChance,
			},
		},
		City: api.CityInfo{
			Name: todayData.Location,
			Coord: api.Coordinates{
				Lat: oneCall.Lat,
				Lon: oneCall.Lon,
			},
			Timezone: oneCall.TimezoneOffset,
		},
	}

	// Add sunrise/sunset from daily data if available
	if len(oneCall.Daily) > 0 {
		forecast.City.Sunrise = oneCall.Daily[0].Sunrise
		forecast.City.Sunset = oneCall.Daily[0].Sunset
	}

	return forecast
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}
