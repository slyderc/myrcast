package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// APIs contains API key configurations
type APIs struct {
	OpenWeather string `toml:"openweather"`
	Anthropic   string `toml:"anthropic"`
	ElevenLabs  string `toml:"elevenlabs"`
}

// Weather contains weather query configuration
type Weather struct {
	Latitude  float64 `toml:"latitude"`
	Longitude float64 `toml:"longitude"`
	Units     string  `toml:"units"`
}

// Output contains output path configurations
type Output struct {
	ImportPath string `toml:"import_path"`
	MediaID    string `toml:"media_id"` // Base filename for generated audio (without extension)
}

// Prompt contains AI prompt template configuration
type Prompt struct {
	Template string `toml:"template"`
}

// Claude contains Claude AI model configuration
type Claude struct {
	Model       string  `toml:"model"`
	MaxTokens   int     `toml:"max_tokens"`
	Temperature float64 `toml:"temperature"`
	MaxRetries  int     `toml:"max_retries"`
	BaseDelayMs int     `toml:"base_delay_ms"` // Base delay in milliseconds
	MaxDelayMs  int     `toml:"max_delay_ms"`  // Max delay in milliseconds
	RateLimit   int     `toml:"rate_limit"`    // Requests per minute
}

// ElevenLabs contains ElevenLabs API configuration
type ElevenLabs struct {
	VoiceID     string  `toml:"voice_id"`      // ElevenLabs voice ID
	Model       string  `toml:"model"`         // Voice model (e.g., eleven_multilingual_v1)
	Stability   float64 `toml:"stability"`     // Voice stability (0.0-1.0)
	Similarity  float64 `toml:"similarity"`    // Voice similarity boost (0.0-1.0)
	Style       float64 `toml:"style"`         // Style exaggeration (0.0-1.0)
	Speed       float64 `toml:"speed"`         // Speaking speed (0.25-4.0, 1.0 = normal)
	Format      string  `toml:"format"`        // Audio format (e.g., mp3_44100_128)
	MaxRetries  int     `toml:"max_retries"`   // Max retry attempts
	BaseDelayMs int     `toml:"base_delay_ms"` // Base delay in milliseconds
	MaxDelayMs  int     `toml:"max_delay_ms"`  // Max delay in milliseconds
	RateLimit   int     `toml:"rate_limit"`    // Requests per minute
}

// Logging contains logging configuration with rotation and cross-platform support
type Logging struct {
	Enabled         bool   `toml:"enabled"`          // Enable file logging
	Directory       string `toml:"directory"`        // Log directory (relative or absolute)
	FilenamePattern string `toml:"filename_pattern"` // Log filename with date patterns
	Level           string `toml:"level"`            // Log level: debug, info, warn, error
	MaxFiles        int    `toml:"max_files"`        // Number of log files to keep
	MaxSizeMB       int    `toml:"max_size_mb"`      // Rotate when file exceeds this size
	ConsoleOutput   bool   `toml:"console_output"`   // Also output to console
}

// Cache contains weather data caching configuration
type Cache struct {
	FilePath string `toml:"file_path"` // Path to weather cache file (JSON format)
}

// Config represents the complete application configuration
type Config struct {
	APIs       APIs       `toml:"apis"`
	Weather    Weather    `toml:"weather"`
	Output     Output     `toml:"output"`
	Prompt     Prompt     `toml:"prompt"`
	Claude     Claude     `toml:"claude"`
	ElevenLabs ElevenLabs `toml:"elevenlabs"`
	Logging    Logging    `toml:"logging"`
	Cache      Cache      `toml:"cache"`
}

// LoadConfig reads and parses a TOML configuration file
func LoadConfig(configPath string) (*Config, error) {
	// Clean the path to handle both Windows and Unix paths
	cleanPath := filepath.Clean(configPath)

	// Read the TOML file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ConfigNotFoundError{
				Path: cleanPath,
			}
		}
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse TOML into Config struct
	var config Config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TOML configuration: %w", err)
	}

	// Apply default values
	config.ApplyDefaults()

	return &config, nil
}

// ApplyDefaults sets default values for optional configuration fields
func (c *Config) ApplyDefaults() {
	// Default weather units
	if strings.TrimSpace(c.Weather.Units) == "" {
		c.Weather.Units = "imperial"
	}

	// Default import path
	if strings.TrimSpace(c.Output.ImportPath) == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			c.Output.ImportPath = filepath.Join(os.TempDir(), "myrcast-import")
		} else {
			c.Output.ImportPath = filepath.Join(homeDir, "Documents", "Myrcast")
		}
	}

	// Note: MediaID is required - no default value provided

	// Default prompt template
	if strings.TrimSpace(c.Prompt.Template) == "" {
		c.Prompt.Template = "You are a professional radio weather announcer for morning drive time. Generate a 20-second weather report that's upbeat and informative. Include current conditions, today's high and low temperatures, and any weather to watch for. Use conversational language that sounds natural when spoken aloud. Keep it concise and engaging for busy commuters."
	}

	// Default Claude settings
	if strings.TrimSpace(c.Claude.Model) == "" {
		c.Claude.Model = "claude-3-5-sonnet-20241022"
	}
	if c.Claude.MaxTokens <= 0 {
		c.Claude.MaxTokens = 1000
	}
	if c.Claude.Temperature <= 0 {
		c.Claude.Temperature = 0.7
	}

	// Default ElevenLabs settings
	if strings.TrimSpace(c.ElevenLabs.VoiceID) == "" {
		c.ElevenLabs.VoiceID = "pNInz6obpgDQGcFmaJgB" // Default Adam voice
	}
	if strings.TrimSpace(c.ElevenLabs.Model) == "" {
		c.ElevenLabs.Model = "eleven_multilingual_v1"
	}
	if c.ElevenLabs.Stability <= 0 {
		c.ElevenLabs.Stability = 0.5
	}
	if c.ElevenLabs.Similarity <= 0 {
		c.ElevenLabs.Similarity = 0.8
	}
	if c.ElevenLabs.Style <= 0 {
		c.ElevenLabs.Style = 0.0
	}
	if c.ElevenLabs.Speed <= 0 {
		c.ElevenLabs.Speed = 1.0
	}
	if strings.TrimSpace(c.ElevenLabs.Format) == "" {
		c.ElevenLabs.Format = "mp3_44100_128"
	}
	if c.ElevenLabs.MaxRetries <= 0 {
		c.ElevenLabs.MaxRetries = 3
	}
	if c.ElevenLabs.BaseDelayMs <= 0 {
		c.ElevenLabs.BaseDelayMs = 1000
	}
	if c.ElevenLabs.MaxDelayMs <= 0 {
		c.ElevenLabs.MaxDelayMs = 30000
	}
	if c.ElevenLabs.RateLimit <= 0 {
		c.ElevenLabs.RateLimit = 20 // Conservative rate limit for ElevenLabs
	}

	// Default logging settings
	// Enable logging by default for production use
	// Note: Explicitly set to false in config if you don't want logging

	if strings.TrimSpace(c.Logging.Directory) == "" {
		c.Logging.Directory = "logs"
	}
	if strings.TrimSpace(c.Logging.FilenamePattern) == "" {
		c.Logging.FilenamePattern = "myrcast-YYYYMMDD.log"
	}
	if strings.TrimSpace(c.Logging.Level) == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.MaxFiles <= 0 {
		c.Logging.MaxFiles = 7 // Keep 7 days of logs by default
	}
	if c.Logging.MaxSizeMB <= 0 {
		c.Logging.MaxSizeMB = 10 // 10MB default rotation size
	}
	// ConsoleOutput defaults to false for production, can be enabled in config

	// Default cache settings
	if strings.TrimSpace(c.Cache.FilePath) == "" {
		// Use system temp directory for cross-platform compatibility
		c.Cache.FilePath = filepath.Join(os.TempDir(), "myrcast-weather-cache.toml")
	}
}

// ConfigNotFoundError represents a missing configuration file
type ConfigNotFoundError struct {
	Path string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("configuration file not found: %s\n\nTo create a sample configuration file, run:\n  %s --generate-config", e.Path, filepath.Base(os.Args[0]))
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// Validate checks the configuration for correctness and completeness
func (c *Config) Validate() error {
	var errors []ValidationError

	// Validate API keys
	if err := c.validateAPIKeys(); err != nil {
		errors = append(errors, err...)
	}

	// Validate weather settings
	if err := c.validateWeather(); err != nil {
		errors = append(errors, err...)
	}

	// Validate output settings
	if err := c.validateOutput(); err != nil {
		errors = append(errors, err...)
	}

	// Validate prompt settings
	if err := c.validatePrompt(); err != nil {
		errors = append(errors, err...)
	}

	// Validate Claude settings
	if err := c.validateClaude(); err != nil {
		errors = append(errors, err...)
	}

	// Validate ElevenLabs settings
	if err := c.validateElevenLabs(); err != nil {
		errors = append(errors, err...)
	}

	// Validate logging settings
	if err := c.validateLogging(); err != nil {
		errors = append(errors, err...)
	}

	// Validate cache settings
	if err := c.validateCache(); err != nil {
		errors = append(errors, err...)
	}

	if len(errors) > 0 {
		return &MultiValidationError{Errors: errors}
	}

	return nil
}

// MultiValidationError represents multiple validation errors
type MultiValidationError struct {
	Errors []ValidationError
}

func (e *MultiValidationError) Error() string {
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("configuration validation failed:\n  %s", strings.Join(messages, "\n  "))
}

// validateAPIKeys checks that required API keys are present
func (c *Config) validateAPIKeys() []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(c.APIs.OpenWeather) == "" {
		errors = append(errors, ValidationError{
			Field:   "apis.openweather",
			Message: "OpenWeather API key is required. Get one at https://openweathermap.org/api",
		})
	}

	if strings.TrimSpace(c.APIs.Anthropic) == "" {
		errors = append(errors, ValidationError{
			Field:   "apis.anthropic",
			Message: "Anthropic API key is required. Get one at https://console.anthropic.com/",
		})
	}

	if strings.TrimSpace(c.APIs.ElevenLabs) == "" {
		errors = append(errors, ValidationError{
			Field:   "apis.elevenlabs",
			Message: "ElevenLabs API key is required. Get one at https://elevenlabs.io/",
		})
	}

	return errors
}

// validateWeather checks weather configuration
func (c *Config) validateWeather() []ValidationError {
	var errors []ValidationError

	// Validate latitude range
	if c.Weather.Latitude < -90 || c.Weather.Latitude > 90 {
		errors = append(errors, ValidationError{
			Field:   "weather.latitude",
			Message: fmt.Sprintf("latitude must be between -90 and 90, got %.6f", c.Weather.Latitude),
		})
	}

	// Validate longitude range
	if c.Weather.Longitude < -180 || c.Weather.Longitude > 180 {
		errors = append(errors, ValidationError{
			Field:   "weather.longitude",
			Message: fmt.Sprintf("longitude must be between -180 and 180, got %.6f", c.Weather.Longitude),
		})
	}

	// Validate units
	validUnits := []string{"metric", "imperial", "kelvin"}
	units := strings.ToLower(strings.TrimSpace(c.Weather.Units))
	if units == "" {
		errors = append(errors, ValidationError{
			Field:   "weather.units",
			Message: "units field is required (metric, imperial, or kelvin)",
		})
	} else {
		valid := false
		for _, validUnit := range validUnits {
			if units == validUnit {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ValidationError{
				Field:   "weather.units",
				Message: fmt.Sprintf("units must be one of: %s, got '%s'", strings.Join(validUnits, ", "), c.Weather.Units),
			})
		}
	}

	return errors
}

// validateOutput checks output directory configuration
func (c *Config) validateOutput() []ValidationError {
	var errors []ValidationError

	// Validate import path
	if strings.TrimSpace(c.Output.ImportPath) == "" {
		errors = append(errors, ValidationError{
			Field:   "output.import_path",
			Message: "import path is required",
		})
	}
	if strings.TrimSpace(c.Output.MediaID) == "" {
		errors = append(errors, ValidationError{
			Field:   "output.media_id",
			Message: "media ID is required for audio filename",
		})
	}

	return errors
}

// validatePrompt checks prompt configuration
func (c *Config) validatePrompt() []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(c.Prompt.Template) == "" {
		errors = append(errors, ValidationError{
			Field:   "prompt.template",
			Message: "prompt template is required",
		})
	}

	return errors
}

// validateClaude checks Claude configuration
func (c *Config) validateClaude() []ValidationError {
	var errors []ValidationError

	// Validate model
	if strings.TrimSpace(c.Claude.Model) == "" {
		errors = append(errors, ValidationError{
			Field:   "claude.model",
			Message: "Claude model is required (e.g., claude-3-5-sonnet-20241022)",
		})
	}

	// Validate max tokens
	if c.Claude.MaxTokens < 100 || c.Claude.MaxTokens > 4096 {
		errors = append(errors, ValidationError{
			Field:   "claude.max_tokens",
			Message: fmt.Sprintf("max_tokens must be between 100 and 4096, got %d", c.Claude.MaxTokens),
		})
	}

	// Validate temperature
	if c.Claude.Temperature < 0 || c.Claude.Temperature > 1 {
		errors = append(errors, ValidationError{
			Field:   "claude.temperature",
			Message: fmt.Sprintf("temperature must be between 0 and 1, got %.2f", c.Claude.Temperature),
		})
	}

	return errors
}

// validateElevenLabs checks ElevenLabs configuration
func (c *Config) validateElevenLabs() []ValidationError {
	var errors []ValidationError

	// Validate voice ID
	if strings.TrimSpace(c.ElevenLabs.VoiceID) == "" {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.voice_id",
			Message: "ElevenLabs voice ID is required",
		})
	}

	// Validate model
	if strings.TrimSpace(c.ElevenLabs.Model) == "" {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.model",
			Message: "ElevenLabs model is required (e.g., eleven_multilingual_v1)",
		})
	}

	// Validate stability (0.0-1.0)
	if c.ElevenLabs.Stability < 0 || c.ElevenLabs.Stability > 1 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.stability",
			Message: fmt.Sprintf("stability must be between 0.0 and 1.0, got %.2f", c.ElevenLabs.Stability),
		})
	}

	// Validate similarity (0.0-1.0)
	if c.ElevenLabs.Similarity < 0 || c.ElevenLabs.Similarity > 1 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.similarity",
			Message: fmt.Sprintf("similarity must be between 0.0 and 1.0, got %.2f", c.ElevenLabs.Similarity),
		})
	}

	// Validate style (0.0-1.0)
	if c.ElevenLabs.Style < 0 || c.ElevenLabs.Style > 1 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.style",
			Message: fmt.Sprintf("style must be between 0.0 and 1.0, got %.2f", c.ElevenLabs.Style),
		})
	}

	// Validate speed (0.7-1.2 per ElevenLabs API constraints)
	if c.ElevenLabs.Speed < 0.7 || c.ElevenLabs.Speed > 1.2 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.speed",
			Message: fmt.Sprintf("speed must be between 0.7 and 1.2 (ElevenLabs API constraint), got %.2f", c.ElevenLabs.Speed),
		})
	}

	// Validate format (ElevenLabs format: codec_samplerate_bitrate)
	format := strings.TrimSpace(c.ElevenLabs.Format)
	if format == "" {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.format",
			Message: "audio format is required (ElevenLabs format: codec_samplerate_bitrate, e.g., mp3_44100_128)",
		})
	} else {
		// Basic validation for ElevenLabs format pattern
		if !isValidElevenLabsFormat(format) {
			errors = append(errors, ValidationError{
				Field:   "elevenlabs.format",
				Message: fmt.Sprintf("format must be ElevenLabs format (codec_samplerate_bitrate), got '%s'. See: https://elevenlabs.io/docs/api-reference/text-to-speech/convert", c.ElevenLabs.Format),
			})
		}
	}

	// Validate retry settings
	if c.ElevenLabs.MaxRetries < 0 || c.ElevenLabs.MaxRetries > 10 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.max_retries",
			Message: fmt.Sprintf("max_retries must be between 0 and 10, got %d", c.ElevenLabs.MaxRetries),
		})
	}

	// Validate rate limit
	if c.ElevenLabs.RateLimit < 1 || c.ElevenLabs.RateLimit > 100 {
		errors = append(errors, ValidationError{
			Field:   "elevenlabs.rate_limit",
			Message: fmt.Sprintf("rate_limit must be between 1 and 100 requests per minute, got %d", c.ElevenLabs.RateLimit),
		})
	}

	return errors
}

// validateLogging checks logging configuration
func (c *Config) validateLogging() []ValidationError {
	var errors []ValidationError

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	level := strings.ToLower(strings.TrimSpace(c.Logging.Level))
	if level != "" {
		valid := false
		for _, validLevel := range validLevels {
			if level == validLevel {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ValidationError{
				Field:   "logging.level",
				Message: fmt.Sprintf("level must be one of: %s, got '%s'", strings.Join(validLevels, ", "), c.Logging.Level),
			})
		}
	}

	// Validate max files
	if c.Logging.MaxFiles < 0 || c.Logging.MaxFiles > 365 {
		errors = append(errors, ValidationError{
			Field:   "logging.max_files",
			Message: fmt.Sprintf("max_files must be between 0 and 365, got %d", c.Logging.MaxFiles),
		})
	}

	// Validate max size
	if c.Logging.MaxSizeMB < 0 || c.Logging.MaxSizeMB > 1000 {
		errors = append(errors, ValidationError{
			Field:   "logging.max_size_mb",
			Message: fmt.Sprintf("max_size_mb must be between 0 and 1000, got %d", c.Logging.MaxSizeMB),
		})
	}

	// Validate directory if logging is enabled
	if c.Logging.Enabled {
		if strings.TrimSpace(c.Logging.Directory) == "" {
			errors = append(errors, ValidationError{
				Field:   "logging.directory",
				Message: "directory is required when logging is enabled",
			})
		}

		if strings.TrimSpace(c.Logging.FilenamePattern) == "" {
			errors = append(errors, ValidationError{
				Field:   "logging.filename_pattern",
				Message: "filename_pattern is required when logging is enabled",
			})
		} else {
			// Note: We can't import internal/logger from config package due to circular dependency
			// The validation will be done in the logger package when initializing
			// This is intentional to keep config package independent
		}
	}

	return errors
}

// validateCache checks cache configuration
func (c *Config) validateCache() []ValidationError {
	var errors []ValidationError

	// Validate cache file path (only if specified, empty uses default)
	if strings.TrimSpace(c.Cache.FilePath) != "" {
		// Ensure the parent directory exists and is writable
		cacheDir := filepath.Dir(c.Cache.FilePath)
		if cacheDir != "." && cacheDir != "" {
			// Only create parent directory if it's not the current directory
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				errors = append(errors, ValidationError{
					Field:   "cache.file_path",
					Message: fmt.Sprintf("cannot create cache directory: %v", err),
				})
			}
		}

		// Test if we can write to the cache file location
		testFile := c.Cache.FilePath + ".test"
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			errors = append(errors, ValidationError{
				Field:   "cache.file_path",
				Message: fmt.Sprintf("cache file location is not writable: %v", err),
			})
		} else {
			// Clean up test file
			os.Remove(testFile)
		}
	}

	return errors
}

// GenerateSampleConfig creates a sample configuration file at the specified path
func GenerateSampleConfig(configPath string) error {
	sampleConfig := `# Myrcast Configuration File
# Weather Report Generator with AI and Speech

[apis]
# Get your OpenWeather API key at: https://openweathermap.org/api
openweather = "your-openweather-api-key-here"

# Get your Anthropic API key at: https://console.anthropic.com/
anthropic = "your-anthropic-api-key-here"

# Get your ElevenLabs API key at: https://elevenlabs.io/
elevenlabs = "your-elevenlabs-api-key-here"

[weather]
# Coordinates for your location (example: San Francisco)
latitude = 37.7749
longitude = -122.4194

# Units: "metric", "imperial", or "kelvin"
units = "imperial"

[output]
# Directory where Myriad should import generated content
import_path = "/Users/username/Documents/Myrcast"

# Base filename for generated audio files (without extension)
# The .wav extension will be added automatically
media_id = "weather_report"

[prompt]
# Template for AI weather report generation
# Describe the style, tone, and format you want for your weather reports
# Claude will automatically extract relevant details from the weather data provided
template = "You are a professional radio weather announcer for morning drive time. Generate a 20-second weather report that's upbeat and informative. Include current conditions, today's high and low temperatures, and any weather to watch for. Use conversational language that sounds natural when spoken aloud. Keep it concise and engaging for busy commuters."

[claude]
# Claude model to use (defaults to claude-3-5-sonnet-20241022)
model = "claude-3-5-sonnet-20241022"

# Maximum tokens to generate (100-4096)
max_tokens = 1000

# Temperature for response generation (0-1, higher = more creative)
temperature = 0.7

[elevenlabs]
# Voice ID from ElevenLabs (find at https://elevenlabs.io/voice-library)
voice_id = "pNInz6obpgDQGcFmaJgB"

# Voice model to use
model = "eleven_multilingual_v1"

# Voice stability (0.0-1.0, higher = more stable/consistent)
stability = 0.5

# Similarity boost (0.0-1.0, higher = more similar to original voice)
similarity = 0.8

# Style exaggeration (0.0-1.0, higher = more expressive)
style = 0.0

# Speaking speed (0.7-1.2, higher = faster speech)
# 1.0 is normal speed, ElevenLabs enforces 0.7-1.2 range
speed = 1.0

# Audio format: ElevenLabs format (codec_samplerate_bitrate)
# Examples: mp3_44100_128, pcm_16000, ulaw_8000
format = "mp3_44100_128"

# Retry settings for API failures
max_retries = 3
base_delay_ms = 1000
max_delay_ms = 30000

# Rate limiting (requests per minute)
rate_limit = 20

[logging]
# Cross-platform file logging with rotation and enhanced validation
# Essential for production use and debugging
enabled = true                              # Enable file logging
directory = "logs"                          # Log directory (relative to working dir or absolute path)
                                           # Windows default: %APPDATA%\Myrcast\logs  
                                           # macOS default: ~/.myrcast/logs
filename_pattern = "myrcast-YYYYMMDD.log"  # Daily rotation pattern
                                           # YYYY=year, MM=month, DD=day, HH=hour, MM=minute
level = "info"                             # Log level: debug, info, warn, error
max_files = 7                              # Keep 7 days of logs (0 = unlimited)
max_size_mb = 10                           # Rotate when file exceeds 10MB (0 = unlimited)  
console_output = true                      # Also output to console (helpful for debugging)

[cache]
# Weather data caching configuration
# Reduces API calls by caching daily forecast data
# Cache automatically expires at midnight local time
# Current conditions are always fetched fresh from the API
file_path = ""                             # Path to weather cache file (leave empty for default)
                                           # Default: system temp directory
                                           # Windows: %TEMP%\myrcast-weather-cache.toml
                                           # macOS/Linux: /tmp/myrcast-weather-cache.toml
`

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write sample config
	if err := os.WriteFile(configPath, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to write sample config: %w", err)
	}

	return nil
}

// isValidElevenLabsFormat validates the ElevenLabs audio format pattern
func isValidElevenLabsFormat(format string) bool {
	// ElevenLabs format pattern: codec_samplerate_bitrate
	// Examples: mp3_44100_128, pcm_16000, ulaw_8000

	// Common valid formats based on ElevenLabs documentation
	validFormats := []string{
		"mp3_44100_128", "mp3_44100_192", "mp3_44100_64",
		"pcm_16000", "pcm_22050", "pcm_24000", "pcm_44100",
		"ulaw_8000",
	}

	// Check exact matches first
	for _, valid := range validFormats {
		if format == valid {
			return true
		}
	}

	// Basic pattern validation for unknown formats
	// Pattern: word_digits or word_digits_digits
	pattern := regexp.MustCompile(`^[a-z]+_\d+(_\d+)?$`)
	return pattern.MatchString(format)
}
