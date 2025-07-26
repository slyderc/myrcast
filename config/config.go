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
}

// Weather contains weather query configuration
type Weather struct {
	Latitude  float64 `toml:"latitude"`
	Longitude float64 `toml:"longitude"`
	Units     string  `toml:"units"`
}

// Output contains output path configurations
type Output struct {
	TempDirectory string `toml:"temp_directory"`
	ImportPath    string `toml:"import_path"`
}

// Speech contains text-to-speech configuration
type Speech struct {
	Voice  string  `toml:"voice"`
	Speed  float64 `toml:"speed"`
	Format string  `toml:"format"`
}

// Prompt contains AI prompt template configuration
type Prompt struct {
	Template string `toml:"template"`
}

// Config represents the complete application configuration
type Config struct {
	APIs    APIs    `toml:"apis"`
	Weather Weather `toml:"weather"`
	Output  Output  `toml:"output"`
	Speech  Speech  `toml:"speech"`
	Prompt  Prompt  `toml:"prompt"`
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
	
	// Default temp directory
	if strings.TrimSpace(c.Output.TempDirectory) == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			c.Output.TempDirectory = filepath.Join(os.TempDir(), "myrcast")
		} else {
			c.Output.TempDirectory = filepath.Join(homeDir, ".myrcast", "temp")
		}
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
	
	// Default speech settings
	if strings.TrimSpace(c.Speech.Voice) == "" {
		c.Speech.Voice = "alloy"
	}
	if c.Speech.Speed <= 0 {
		c.Speech.Speed = 1.0
	}
	if strings.TrimSpace(c.Speech.Format) == "" {
		c.Speech.Format = "mp3_44100_128"
	}
	
	// Default prompt template
	if strings.TrimSpace(c.Prompt.Template) == "" {
		c.Prompt.Template = "You are a friendly weather reporter. Generate an engaging weather report for the location based on the provided weather data. Include current conditions, temperature, and any notable weather patterns. Keep it conversational and informative."
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
	
	// Validate speech settings
	if err := c.validateSpeech(); err != nil {
		errors = append(errors, err...)
	}
	
	// Validate prompt settings
	if err := c.validatePrompt(); err != nil {
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
	
	// Validate temp directory
	if strings.TrimSpace(c.Output.TempDirectory) == "" {
		errors = append(errors, ValidationError{
			Field:   "output.temp_directory",
			Message: "temp directory path is required",
		})
	} else {
		// Check if directory exists or can be created
		tempDir := filepath.Clean(c.Output.TempDirectory)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			errors = append(errors, ValidationError{
				Field:   "output.temp_directory",
				Message: fmt.Sprintf("cannot create temp directory: %v", err),
			})
		} else {
			// Check if directory is writable
			testFile := filepath.Join(tempDir, ".write_test")
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				errors = append(errors, ValidationError{
					Field:   "output.temp_directory",
					Message: fmt.Sprintf("temp directory is not writable: %v", err),
				})
			} else {
				// Clean up test file
				os.Remove(testFile)
			}
		}
	}
	
	// Validate import path
	if strings.TrimSpace(c.Output.ImportPath) == "" {
		errors = append(errors, ValidationError{
			Field:   "output.import_path",
			Message: "import path is required",
		})
	}
	
	return errors
}

// validateSpeech checks speech configuration
func (c *Config) validateSpeech() []ValidationError {
	var errors []ValidationError
	
	// Validate voice (any non-empty string is valid, ElevenLabs API will validate)
	voice := strings.TrimSpace(c.Speech.Voice)
	if voice == "" {
		errors = append(errors, ValidationError{
			Field:   "speech.voice",
			Message: "voice is required (will be validated by ElevenLabs API)",
		})
	}
	
	// Validate speed
	if c.Speech.Speed <= 0 || c.Speech.Speed > 4.0 {
		errors = append(errors, ValidationError{
			Field:   "speech.speed",
			Message: fmt.Sprintf("speech speed must be between 0.1 and 4.0, got %.2f", c.Speech.Speed),
		})
	}
	
	// Validate format (ElevenLabs format: codec_samplerate_bitrate)
	format := strings.TrimSpace(c.Speech.Format)
	if format == "" {
		errors = append(errors, ValidationError{
			Field:   "speech.format",
			Message: "audio format is required (ElevenLabs format: codec_samplerate_bitrate, e.g., mp3_44100_128)",
		})
	} else {
		// Basic validation for ElevenLabs format pattern
		if !isValidElevenLabsFormat(format) {
			errors = append(errors, ValidationError{
				Field:   "speech.format",
				Message: fmt.Sprintf("format must be ElevenLabs format (codec_samplerate_bitrate), got '%s'. See: https://elevenlabs.io/docs/api-reference/text-to-speech/convert", c.Speech.Format),
			})
		}
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

// GenerateSampleConfig creates a sample configuration file at the specified path
func GenerateSampleConfig(configPath string) error {
	sampleConfig := `# Myrcast Configuration File
# Weather Report Generator with AI and Speech

[apis]
# Get your OpenWeather API key at: https://openweathermap.org/api
openweather = "your-openweather-api-key-here"

# Get your Anthropic API key at: https://console.anthropic.com/
anthropic = "your-anthropic-api-key-here"

[weather]
# Coordinates for your location (example: San Francisco)
latitude = 37.7749
longitude = -122.4194

# Units: "metric", "imperial", or "kelvin"
units = "imperial"

[output]
# Directory for temporary files
temp_directory = "/tmp/myrcast"

# Directory where Myriad should import generated content
import_path = "/Users/username/Documents/Myrcast"

[speech]
# Voice ID for ElevenLabs (any voice name or ID)
voice = "alloy"

# Speech speed (0.1 to 4.0)
speed = 1.0

# Audio format: ElevenLabs format (codec_samplerate_bitrate)
# Examples: mp3_44100_128, pcm_16000, ulaw_8000
format = "mp3_44100_128"

[prompt]
# Template for AI weather report generation
template = "You are a friendly weather reporter. Generate an engaging weather report for the location based on the provided weather data. Include current conditions, temperature, and any notable weather patterns. Keep it conversational and informative."
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