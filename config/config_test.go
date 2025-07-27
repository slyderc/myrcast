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
temp_directory = "/tmp/test"
import_path = "/tmp/test/import"

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
			speed:     0.25,
			wantError: false,
		},
		{
			name:      "Valid speed - maximum",
			speed:     4.0,
			wantError: false,
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
temp_directory = "/tmp/test"
import_path = "/tmp/test/import"

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
					TempDirectory: "/tmp/test",
					ImportPath:    "/tmp/test/import",
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
temp_directory = "/tmp/test"
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

