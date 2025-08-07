package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigurationConsolidation tests that the Speech section has been properly consolidated into ElevenLabs
func TestConfigurationConsolidation(t *testing.T) {
	// Create a temporary config file with the new consolidated structure
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.toml")

	configContent := `[apis]
openweather = "test-weather-key"
anthropic = "test-anthropic-key"
elevenlabs = "test-elevenlabs-key"

[weather]
latitude = 37.7749
longitude = -122.4194
units = "imperial"

[output]
import_path = "/tmp/test/import"
media_id = "test_report"

[prompt]
template = "Test weather report template"

[claude]
model = "claude-3-5-sonnet-20241022"
max_tokens = 1000
temperature = 0.7

[elevenlabs]
voice_id = "test-voice-id"
model = "eleven_multilingual_v1"
stability = 0.6
similarity = 0.9
style = 0.1
speed = 1.5
format = "mp3_44100_128"
max_retries = 5
base_delay_ms = 2000
max_delay_ms = 60000
rate_limit = 10
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify ElevenLabs consolidation
	if cfg.ElevenLabs.VoiceID != "test-voice-id" {
		t.Errorf("Expected voice_id 'test-voice-id', got '%s'", cfg.ElevenLabs.VoiceID)
	}

	if cfg.ElevenLabs.Speed != 1.5 {
		t.Errorf("Expected speed 1.5, got %f", cfg.ElevenLabs.Speed)
	}

	if cfg.ElevenLabs.Format != "mp3_44100_128" {
		t.Errorf("Expected format 'mp3_44100_128', got '%s'", cfg.ElevenLabs.Format)
	}

	if cfg.ElevenLabs.Stability != 0.6 {
		t.Errorf("Expected stability 0.6, got %f", cfg.ElevenLabs.Stability)
	}

	if cfg.ElevenLabs.Similarity != 0.9 {
		t.Errorf("Expected similarity 0.9, got %f", cfg.ElevenLabs.Similarity)
	}
}

// TestElevenLabsSpeedValidation tests speed parameter validation
func TestElevenLabsSpeedValidation(t *testing.T) {
	tests := []struct {
		name      string
		speed     float64
		wantError bool
	}{
		{
			name:      "Valid speed - normal",
			speed:     1.0,
			wantError: false,
		},
		{
			name:      "Valid speed - minimum",
			speed:     0.7,
			wantError: false,
		},
		{
			name:      "Valid speed - maximum",
			speed:     1.2,
			wantError: false,
		},
		{
			name:      "Invalid speed - too low",
			speed:     0.25,
			wantError: true,
		},
		{
			name:      "Invalid speed - too high",
			speed:     4.0,
			wantError: true,
		},
		{
			name:      "Invalid speed - too slow",
			speed:     0.1,
			wantError: true,
		},
		{
			name:      "Invalid speed - too fast",
			speed:     5.0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.toml")

			configContent := fmt.Sprintf(`[apis]
openweather = "test-weather-key"
anthropic = "test-anthropic-key"
elevenlabs = "test-elevenlabs-key"

[weather]
latitude = 37.7749
longitude = -122.4194
units = "imperial"

[output]
import_path = "/tmp/test/import"
media_id = "test_report"

[prompt]
template = "Test weather report template"

[claude]
model = "claude-3-5-sonnet-20241022"
max_tokens = 1000
temperature = 0.7

[elevenlabs]
voice_id = "test-voice-id"
model = "eleven_multilingual_v1"
stability = 0.5
similarity = 0.8
style = 0.0
speed = %g
format = "mp3_44100_128"
max_retries = 3
base_delay_ms = 1000
max_delay_ms = 30000
rate_limit = 20
`, tt.speed)

			err := os.WriteFile(configPath, []byte(configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Load configuration
			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Validate configuration
			err = cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.wantError && err != nil {
				// Check that error mentions speed validation
				if !strings.Contains(err.Error(), "speed must be between") {
					t.Errorf("Expected speed validation error, got: %v", err)
				}
			}
		})
	}
}

// TestElevenLabsFormatValidation tests format parameter validation
func TestElevenLabsFormatValidation(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantError bool
	}{
		{
			name:      "Valid format - mp3_44100_128",
			format:    "mp3_44100_128",
			wantError: false,
		},
		{
			name:      "Valid format - pcm_16000",
			format:    "pcm_16000",
			wantError: false,
		},
		{
			name:      "Valid format - ulaw_8000",
			format:    "ulaw_8000",
			wantError: false,
		},
		{
			name:      "Invalid format - empty",
			format:    "",
			wantError: true,
		},
		{
			name:      "Invalid format - wrong pattern",
			format:    "invalid-format",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				APIs: APIs{
					OpenWeather: "test-key",
					Anthropic:   "test-key",
					ElevenLabs:  "test-key",
				},
				Weather: Weather{
					Latitude:  37.7749,
					Longitude: -122.4194,
					Units:     "imperial",
				},
				Output: Output{
					ImportPath: "/tmp/test/import",
					MediaID:    "test_report",
				},
				Prompt: Prompt{
					Template: "Test template",
				},
				Claude: Claude{
					Model:       "claude-3-5-sonnet-20241022",
					MaxTokens:   1000,
					Temperature: 0.7,
				},
				ElevenLabs: ElevenLabs{
					VoiceID:     "test-voice-id",
					Model:       "eleven_multilingual_v1",
					Stability:   0.5,
					Similarity:  0.8,
					Style:       0.0,
					Speed:       1.0,
					Format:      tt.format,
					MaxRetries:  3,
					BaseDelayMs: 1000,
					MaxDelayMs:  30000,
					RateLimit:   20,
				},
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.wantError && err != nil {
				// Check that error mentions format validation
				if !strings.Contains(err.Error(), "format") {
					t.Errorf("Expected format validation error, got: %v", err)
				}
			}
		})
	}
}

// TestConfigDefaults tests that default values are properly applied
func TestConfigDefaults(t *testing.T) {
	// Create minimal config without optional fields
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.toml")

	minimalConfig := `[apis]
openweather = "test-weather-key"
anthropic = "test-anthropic-key"
elevenlabs = "test-elevenlabs-key"

[weather]
latitude = 37.7749
longitude = -122.4194

[output]
import_path = "/tmp/test"
`

	err := os.WriteFile(configPath, []byte(minimalConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write minimal config: %v", err)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check ElevenLabs defaults
	if cfg.ElevenLabs.VoiceID != "pNInz6obpgDQGcFmaJgB" {
		t.Errorf("Expected default voice ID, got '%s'", cfg.ElevenLabs.VoiceID)
	}

	if cfg.ElevenLabs.Speed != 1.0 {
		t.Errorf("Expected default speed 1.0, got %f", cfg.ElevenLabs.Speed)
	}

	if cfg.ElevenLabs.Format != "mp3_44100_128" {
		t.Errorf("Expected default format 'mp3_44100_128', got '%s'", cfg.ElevenLabs.Format)
	}

	if cfg.ElevenLabs.Stability != 0.5 {
		t.Errorf("Expected default stability 0.5, got %f", cfg.ElevenLabs.Stability)
	}

	if cfg.ElevenLabs.Similarity != 0.8 {
		t.Errorf("Expected default similarity 0.8, got %f", cfg.ElevenLabs.Similarity)
	}

	if cfg.ElevenLabs.Style != 0.0 {
		t.Errorf("Expected default style 0.0, got %f", cfg.ElevenLabs.Style)
	}
}

// TestLoggingConfiguration tests the logging configuration section
func TestLoggingConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		loggingConfig  string
		expectEnabled  bool
		expectLevel    string
		expectMaxFiles int
		wantError      bool
	}{
		{
			name: "Full logging config",
			loggingConfig: `[logging]
enabled = true
directory = "logs"
filename_pattern = "test-YYYYMMDD.log"
level = "debug"
max_files = 30
max_size_mb = 50
console_output = true`,
			expectEnabled:  true,
			expectLevel:    "debug",
			expectMaxFiles: 30,
			wantError:      false,
		},
		{
			name: "Minimal logging config with defaults",
			loggingConfig: `[logging]
enabled = true`,
			expectEnabled:  true,
			expectLevel:    "info", // Default
			expectMaxFiles: 7,      // Default
			wantError:      false,
		},
		{
			name: "Invalid log level",
			loggingConfig: `[logging]
enabled = true
level = "invalid-level"`,
			expectEnabled: true,
			wantError:     true,
		},
		{
			name: "Invalid max_files",
			loggingConfig: `[logging]
enabled = true
max_files = 500`, // Too high
			expectEnabled: true,
			wantError:     true,
		},
		{
			name: "Invalid max_size_mb",
			loggingConfig: `[logging]
enabled = true
max_size_mb = 2000`, // Too high
			expectEnabled: true,
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.toml")

			// Base config content
			baseConfig := `[apis]
openweather = "test-key"
anthropic = "test-key"
elevenlabs = "test-key"

[weather]
latitude = 37.7749
longitude = -122.4194
units = "imperial"

[output]
import_path = "/tmp/test/import"
media_id = "test_report"

[prompt]
template = "Test template"

[claude]
model = "claude-3-5-sonnet-20241022"
max_tokens = 1000
temperature = 0.7

[elevenlabs]
voice_id = "test-voice-id"
model = "eleven_multilingual_v1"
stability = 0.5
similarity = 0.8
style = 0.0
speed = 1.0
format = "mp3_44100_128"

`
			// Add logging config
			fullConfig := baseConfig + tt.loggingConfig

			err := os.WriteFile(configPath, []byte(fullConfig), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Load configuration
			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Validate configuration
			validationErr := cfg.Validate()
			if (validationErr != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", validationErr, tt.wantError)
			}

			if !tt.wantError {
				// Check loaded values
				if cfg.Logging.Enabled != tt.expectEnabled {
					t.Errorf("Expected enabled=%v, got %v", tt.expectEnabled, cfg.Logging.Enabled)
				}

				if cfg.Logging.Level != tt.expectLevel {
					t.Errorf("Expected level=%s, got %s", tt.expectLevel, cfg.Logging.Level)
				}

				if cfg.Logging.MaxFiles != tt.expectMaxFiles {
					t.Errorf("Expected max_files=%d, got %d", tt.expectMaxFiles, cfg.Logging.MaxFiles)
				}
			}
		})
	}
}

// TestLoggingDefaults tests that logging defaults are properly applied
func TestLoggingDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "minimal.toml")

	// Config without logging section
	minimalConfig := `[apis]
openweather = "test-key"
anthropic = "test-key"
elevenlabs = "test-key"

[weather]
latitude = 37.7749
longitude = -122.4194
units = "imperial"

[output]
import_path = "/tmp/test/import"
media_id = "test_report"

[prompt]
template = "Test template"

[claude]
model = "claude-3-5-sonnet-20241022"

[elevenlabs]
voice_id = "test-voice-id"
`

	err := os.WriteFile(configPath, []byte(minimalConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write minimal config: %v", err)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check logging defaults
	if cfg.Logging.Directory != "logs" {
		t.Errorf("Expected default directory 'logs', got '%s'", cfg.Logging.Directory)
	}

	if cfg.Logging.FilenamePattern != "myrcast-YYYYMMDD.log" {
		t.Errorf("Expected default filename pattern, got '%s'", cfg.Logging.FilenamePattern)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default level 'info', got '%s'", cfg.Logging.Level)
	}

	if cfg.Logging.MaxFiles != 7 {
		t.Errorf("Expected default max_files 7, got %d", cfg.Logging.MaxFiles)
	}

	if cfg.Logging.MaxSizeMB != 10 {
		t.Errorf("Expected default max_size_mb 10, got %d", cfg.Logging.MaxSizeMB)
	}

	// ConsoleOutput should default to false
	if cfg.Logging.ConsoleOutput {
		t.Error("Expected console_output to default to false")
	}
}

// TestLoggingValidationErrors tests specific logging validation errors
func TestLoggingValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		logging     Logging
		expectError string
	}{
		{
			name: "Missing directory when enabled",
			logging: Logging{
				Enabled:         true,
				Directory:       "",
				FilenamePattern: "test.log",
				Level:           "info",
			},
			expectError: "directory is required when logging is enabled",
		},
		{
			name: "Missing filename pattern when enabled",
			logging: Logging{
				Enabled:         true,
				Directory:       "logs",
				FilenamePattern: "",
				Level:           "info",
			},
			expectError: "filename_pattern is required when logging is enabled",
		},
		{
			name: "Invalid log level",
			logging: Logging{
				Enabled:         true,
				Directory:       "logs",
				FilenamePattern: "test.log",
				Level:           "verbose", // Invalid
			},
			expectError: "level must be one of",
		},
		{
			name: "Negative max_files",
			logging: Logging{
				Enabled:         true,
				Directory:       "logs",
				FilenamePattern: "test.log",
				Level:           "info",
				MaxFiles:        -1,
			},
			expectError: "max_files must be between 0 and 365",
		},
		{
			name: "Negative max_size_mb",
			logging: Logging{
				Enabled:         true,
				Directory:       "logs",
				FilenamePattern: "test.log",
				Level:           "info",
				MaxSizeMB:       -10,
			},
			expectError: "max_size_mb must be between 0 and 1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				APIs: APIs{
					OpenWeather: "test-key",
					Anthropic:   "test-key",
					ElevenLabs:  "test-key",
				},
				Weather: Weather{
					Latitude:  37.7749,
					Longitude: -122.4194,
					Units:     "imperial",
				},
				Output: Output{
					ImportPath: "/tmp/test/import",
					MediaID:    "test_report",
				},
				Prompt: Prompt{
					Template: "Test template",
				},
				Claude: Claude{
					Model:       "claude-3-5-sonnet-20241022",
					MaxTokens:   1000,
					Temperature: 0.7,
				},
				ElevenLabs: ElevenLabs{
					VoiceID: "test-voice-id",
					Model:   "eleven_multilingual_v1",
					Speed:   1.0,
					Format:  "mp3_44100_128",
				},
				Logging: tt.logging,
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectError, err)
			}
		})
	}
}
