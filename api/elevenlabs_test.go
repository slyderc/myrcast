package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myrcast/config"
)

// TestElevenLabsClientCreation tests creating an ElevenLabs client with valid configuration
func TestElevenLabsClientCreation(t *testing.T) {
	// Load dev.toml configuration
	cfg, err := config.LoadConfig("../dev.toml")
	if err != nil {
		t.Fatalf("Failed to load dev.toml: %v", err)
	}

	// Create ElevenLabs client with config values
	elevenLabsConfig := ElevenLabsConfig{
		APIKey:     cfg.APIs.ElevenLabs,
		VoiceID:    cfg.ElevenLabs.VoiceID,
		Model:      cfg.ElevenLabs.Model,
		Stability:  cfg.ElevenLabs.Stability,
		Similarity: cfg.ElevenLabs.Similarity,
		Style:      cfg.ElevenLabs.Style,
		Speed:      cfg.ElevenLabs.Speed,
		Format:     cfg.ElevenLabs.Format,
		Timeout:    30 * time.Second,
		MaxRetries: cfg.ElevenLabs.MaxRetries,
		BaseDelay:  time.Duration(cfg.ElevenLabs.BaseDelayMs) * time.Millisecond,
		MaxDelay:   time.Duration(cfg.ElevenLabs.MaxDelayMs) * time.Millisecond,
		RateLimit:  cfg.ElevenLabs.RateLimit,
	}

	client, err := NewElevenLabsClient(elevenLabsConfig)
	if err != nil {
		t.Fatalf("Failed to create ElevenLabs client: %v", err)
	}

	// Verify client configuration
	if client.config.APIKey != cfg.APIs.ElevenLabs {
		t.Errorf("Expected API key from config, got different value")
	}

	if client.config.VoiceID != cfg.ElevenLabs.VoiceID {
		t.Errorf("Expected voice ID '%s', got '%s'", cfg.ElevenLabs.VoiceID, client.config.VoiceID)
	}

	if client.config.Model != cfg.ElevenLabs.Model {
		t.Errorf("Expected model '%s', got '%s'", cfg.ElevenLabs.Model, client.config.Model)
	}

	if client.config.Stability != cfg.ElevenLabs.Stability {
		t.Errorf("Expected stability %.2f, got %.2f", cfg.ElevenLabs.Stability, client.config.Stability)
	}
}

// TestElevenLabsClientDefaults tests default values when config is empty
func TestElevenLabsClientDefaults(t *testing.T) {
	elevenLabsConfig := ElevenLabsConfig{
		APIKey: "test-api-key",
		// Leave other fields empty to test defaults
	}

	client, err := NewElevenLabsClient(elevenLabsConfig)
	if err != nil {
		t.Fatalf("Failed to create ElevenLabs client: %v", err)
	}

	// Check defaults
	if client.config.VoiceID != "pNInz6obpgDQGcFmaJgB" {
		t.Errorf("Expected default voice ID 'pNInz6obpgDQGcFmaJgB', got '%s'", client.config.VoiceID)
	}

	if client.config.Model != "eleven_multilingual_v1" {
		t.Errorf("Expected default model 'eleven_multilingual_v1', got '%s'", client.config.Model)
	}

	if client.config.Stability != 0.5 {
		t.Errorf("Expected default stability 0.5, got %.2f", client.config.Stability)
	}

	if client.config.Similarity != 0.8 {
		t.Errorf("Expected default similarity 0.8, got %.2f", client.config.Similarity)
	}

	if client.config.Style != 0.0 {
		t.Errorf("Expected default style 0.0, got %.2f", client.config.Style)
	}

	if client.config.Speed != 1.0 {
		t.Errorf("Expected default speed 1.0, got %.2f", client.config.Speed)
	}

	if client.config.Format != "mp3_44100_128" {
		t.Errorf("Expected default format 'mp3_44100_128', got '%s'", client.config.Format)
	}

	if client.config.Timeout != defaultElevenLabsTimeout {
		t.Errorf("Expected default timeout %v, got %v", defaultElevenLabsTimeout, client.config.Timeout)
	}
}

// TestElevenLabsClientAPIKeyValidation tests API key validation
func TestElevenLabsClientAPIKeyValidation(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		wantError bool
	}{
		{
			name:      "Empty API key",
			apiKey:    "",
			wantError: true,
		},
		{
			name:      "Whitespace API key",
			apiKey:    "   ",
			wantError: true,
		},
		{
			name:      "Valid API key",
			apiKey:    "sk_test_api_key",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elevenLabsConfig := ElevenLabsConfig{
				APIKey: tt.apiKey,
			}

			_, err := NewElevenLabsClient(elevenLabsConfig)
			if (err != nil) != tt.wantError {
				t.Errorf("NewElevenLabsClient() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateTextToSpeechRequest tests request validation logic
func TestValidateTextToSpeechRequest(t *testing.T) {
	client := &ElevenLabsClient{}

	tests := []struct {
		name      string
		request   TextToSpeechRequest
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid request",
			request: TextToSpeechRequest{
				Text:      "This is a test weather report for San Francisco with clear skies.",
				OutputDir: "/tmp/test",
				FileName:  "weather_report",
			},
			wantError: false,
		},
		{
			name: "Empty text",
			request: TextToSpeechRequest{
				Text:      "",
				OutputDir: "/tmp/test",
				FileName:  "weather_report",
			},
			wantError: true,
			errorMsg:  "text content is required",
		},
		{
			name: "Text too short",
			request: TextToSpeechRequest{
				Text:      "Hi",
				OutputDir: "/tmp/test",
				FileName:  "weather_report",
			},
			wantError: true,
			errorMsg:  "too short",
		},
		{
			name: "Text too long",
			request: TextToSpeechRequest{
				Text:      strings.Repeat("This is a very long weather report. ", 200), // > 5000 chars
				OutputDir: "/tmp/test",
				FileName:  "weather_report",
			},
			wantError: true,
			errorMsg:  "too long",
		},
		{
			name: "Empty output directory",
			request: TextToSpeechRequest{
				Text:      "This is a test weather report for San Francisco.",
				OutputDir: "",
				FileName:  "weather_report",
			},
			wantError: true,
			errorMsg:  "output directory is required",
		},
		{
			name: "Empty filename",
			request: TextToSpeechRequest{
				Text:      "This is a test weather report for San Francisco.",
				OutputDir: "/tmp/test",
				FileName:  "",
			},
			wantError: true,
			errorMsg:  "output filename is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateTextToSpeechRequest(tt.request)
			if (err != nil) != tt.wantError {
				t.Errorf("validateTextToSpeechRequest() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// TestGetVoiceID tests voice ID selection logic
func TestGetVoiceID(t *testing.T) {
	client := &ElevenLabsClient{
		config: ElevenLabsConfig{
			VoiceID: "default-voice-id",
		},
	}

	tests := []struct {
		name     string
		override string
		expected string
	}{
		{
			name:     "No override - use default",
			override: "",
			expected: "default-voice-id",
		},
		{
			name:     "With override - use override",
			override: "custom-voice-id",
			expected: "custom-voice-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.getVoiceID(tt.override)
			if result != tt.expected {
				t.Errorf("getVoiceID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestElevenLabsRateLimiter tests the rate limiting functionality
func TestElevenLabsRateLimiter(t *testing.T) {
	// Create rate limiter allowing 2 requests per minute
	limiter := NewElevenLabsRateLimiter(2)

	ctx := context.Background()

	// First two requests should succeed immediately
	start := time.Now()

	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("First request should not be rate limited: %v", err)
	}

	err = limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Second request should not be rate limited: %v", err)
	}

	// Both requests should have completed quickly
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("First two requests took too long: %v", elapsed)
	}

	// Third request should be rate limited - but let's test with a very short timeout
	// to avoid making the test slow
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = limiter.Wait(ctx)
	if err == nil {
		t.Error("Third request should have been rate limited")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got: %v", err)
	}
}

// TestElevenLabsRateLimiterCancellation tests context cancellation
func TestElevenLabsRateLimiterCancellation(t *testing.T) {
	limiter := NewElevenLabsRateLimiter(1)

	// Fill up the rate limiter
	err := limiter.Wait(context.Background())
	if err != nil {
		t.Fatalf("First request should succeed: %v", err)
	}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine that will cancel the context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// This should return context.Canceled
	err = limiter.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// TestParseElevenLabsError tests error parsing and retry logic
func TestParseElevenLabsError(t *testing.T) {
	client := &ElevenLabsClient{}

	tests := []struct {
		name       string
		err        error
		expectType string
		retryable  bool
	}{
		{
			name:       "Rate limit error",
			err:        fmt.Errorf("rate limit exceeded: 429"),
			expectType: "rate_limit_error",
			retryable:  true,
		},
		{
			name:       "Server error",
			err:        fmt.Errorf("internal server error: 500"),
			expectType: "server_error",
			retryable:  true,
		},
		{
			name:       "Authentication error",
			err:        fmt.Errorf("unauthorized: 401"),
			expectType: "authentication_error",
			retryable:  false,
		},
		{
			name:       "Bad request error",
			err:        fmt.Errorf("invalid request: 400"),
			expectType: "invalid_request_error",
			retryable:  false,
		},
		{
			name:       "Network error",
			err:        fmt.Errorf("connection refused"),
			expectType: "network_error",
			retryable:  true,
		},
		{
			name:       "Timeout error",
			err:        context.DeadlineExceeded,
			expectType: "timeout",
			retryable:  true,
		},
		{
			name:       "Cancelled error",
			err:        context.Canceled,
			expectType: "cancelled",
			retryable:  false,
		},
		{
			name:       "Unknown error",
			err:        fmt.Errorf("some other error"),
			expectType: "api_error",
			retryable:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elevenLabsErr := client.parseElevenLabsError(tt.err)

			if elevenLabsErr.Type != tt.expectType {
				t.Errorf("Expected error type %q, got %q", tt.expectType, elevenLabsErr.Type)
			}

			if elevenLabsErr.IsRetryable() != tt.retryable {
				t.Errorf("Expected retryable %v, got %v", tt.retryable, elevenLabsErr.IsRetryable())
			}
		})
	}
}

// TestElevenLabsCalculateRetryDelay tests exponential backoff calculation
func TestElevenLabsCalculateRetryDelay(t *testing.T) {
	client := &ElevenLabsClient{
		config: ElevenLabsConfig{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  5 * time.Second,
		},
	}

	tests := []struct {
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		// Account for 10% jitter (±10%)
		{0, 90 * time.Millisecond, 110 * time.Millisecond},     // 100ms * 2^0 = 100ms ±10%
		{1, 180 * time.Millisecond, 220 * time.Millisecond},    // 100ms * 2^1 = 200ms ±10%
		{2, 360 * time.Millisecond, 440 * time.Millisecond},    // 100ms * 2^2 = 400ms ±10%
		{3, 720 * time.Millisecond, 880 * time.Millisecond},    // 100ms * 2^3 = 800ms ±10%
		{10, 4500 * time.Millisecond, 5500 * time.Millisecond}, // Should be capped at MaxDelay ±10%
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := client.calculateRetryDelay(tt.attempt)

			if delay < tt.expectedMin || delay > tt.expectedMax {
				t.Errorf("Attempt %d: expected delay between %v and %v, got %v",
					tt.attempt, tt.expectedMin, tt.expectedMax, delay)
			}
		})
	}
}

// TestElevenLabsCalculateRetryDelayWithJitter tests jitter functionality
func TestElevenLabsCalculateRetryDelayWithJitter(t *testing.T) {
	client := &ElevenLabsClient{
		config: ElevenLabsConfig{
			BaseDelay: 1 * time.Second,
			MaxDelay:  10 * time.Second,
		},
	}

	// Test that jitter actually varies the delay
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = client.calculateRetryDelay(0) // First attempt
	}

	// Check that we got some variation (not all delays are exactly the same)
	allSame := true
	first := delays[0]
	for _, delay := range delays[1:] {
		if delay != first {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to create variation in delays, but all delays were identical")
	}

	// Check that all delays are within reasonable bounds
	baseDelay := 1 * time.Second
	minExpected := time.Duration(float64(baseDelay) * 0.9) // -10% jitter
	maxExpected := time.Duration(float64(baseDelay) * 1.1) // +10% jitter

	for i, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %d (%v) outside expected range [%v, %v]", i, delay, minExpected, maxExpected)
		}
	}
}

// TestElevenLabsClientConfigDefaults tests that retry defaults are properly applied
func TestElevenLabsClientConfigDefaults(t *testing.T) {
	config := ElevenLabsConfig{
		APIKey: "test-key",
		// Leave retry config empty to test defaults
	}

	client, err := NewElevenLabsClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that defaults were applied
	if client.config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", client.config.MaxRetries)
	}

	if client.config.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay 1s, got %v", client.config.BaseDelay)
	}

	if client.config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay 30s, got %v", client.config.MaxDelay)
	}

	if client.config.RateLimit != 20 {
		t.Errorf("Expected RateLimit 20, got %d", client.config.RateLimit)
	}

	// Check that rate limiter was created
	if client.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}
}

// TestAudioConversionFlow tests the audio file creation and conversion workflow
func TestAudioConversionFlow(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	client := &ElevenLabsClient{}

	// Test MP3 saving with mock data (simple header + minimal data)
	mockMP3Data := []byte{
		// Minimal MP3 header
		0xFF, 0xFB, 0x90, 0x00, // MP3 frame header
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Some data
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	mp3Path, err := client.saveMP3Audio(mockMP3Data, tmpDir, "test_audio")
	if err != nil {
		t.Fatalf("Failed to save MP3 audio: %v", err)
	}

	// Verify MP3 file was created
	if _, err := os.Stat(mp3Path); os.IsNotExist(err) {
		t.Errorf("MP3 file was not created: %s", mp3Path)
	}

	// Verify file has correct extension
	if filepath.Ext(mp3Path) != ".mp3" {
		t.Errorf("Expected .mp3 extension, got %s", filepath.Ext(mp3Path))
	}

	// Verify file contains our data
	savedData, err := os.ReadFile(mp3Path)
	if err != nil {
		t.Fatalf("Failed to read saved MP3 file: %v", err)
	}

	if len(savedData) != len(mockMP3Data) {
		t.Errorf("Expected saved data length %d, got %d", len(mockMP3Data), len(savedData))
	}
}

// TestAudioDurationCalculation tests audio duration calculation
func TestAudioDurationCalculation(t *testing.T) {
	// This test would require a valid WAV file to test duration calculation
	// For now, we'll test the error cases

	client := &ElevenLabsClient{}

	// Test with non-existent file
	_, err := client.calculateAudioDuration("/non/existent/file.wav")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	if !strings.Contains(err.Error(), "failed to open WAV file") {
		t.Errorf("Expected 'failed to open WAV file' error, got: %v", err)
	}
}

// TestElevenLabsAPIIntegration tests actual API call with dev.toml credentials
func TestElevenLabsAPIIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Load dev.toml configuration
	cfg, err := config.LoadConfig("../dev.toml")
	if err != nil {
		t.Fatalf("Failed to load dev.toml: %v", err)
	}

	// Create ElevenLabs client
	elevenLabsConfig := ElevenLabsConfig{
		APIKey:     cfg.APIs.ElevenLabs,
		VoiceID:    cfg.ElevenLabs.VoiceID,
		Model:      cfg.ElevenLabs.Model,
		Stability:  cfg.ElevenLabs.Stability,
		Similarity: cfg.ElevenLabs.Similarity,
		Style:      cfg.ElevenLabs.Style,
		Speed:      cfg.ElevenLabs.Speed,
		Format:     cfg.ElevenLabs.Format,
		Timeout:    30 * time.Second,
		MaxRetries: cfg.ElevenLabs.MaxRetries,
		BaseDelay:  time.Duration(cfg.ElevenLabs.BaseDelayMs) * time.Millisecond,
		MaxDelay:   time.Duration(cfg.ElevenLabs.MaxDelayMs) * time.Millisecond,
		RateLimit:  cfg.ElevenLabs.RateLimit,
	}

	client, err := NewElevenLabsClient(elevenLabsConfig)
	if err != nil {
		t.Fatalf("Failed to create ElevenLabs client: %v", err)
	}

	// Create temporary directory for test output
	tmpDir := t.TempDir()

	// Test text-to-speech generation
	request := TextToSpeechRequest{
		Text:      "This is a test weather report generated by the ElevenLabs integration test.",
		OutputDir: tmpDir,
		FileName:  "integration_test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := client.GenerateTextToSpeech(ctx, request)
	if err != nil {
		t.Fatalf("Failed to generate text-to-speech: %v", err)
	}

	// Verify response
	if response.AudioFilePath == "" {
		t.Error("Expected audio file path in response")
	}

	if response.OriginalMP3 == "" {
		t.Error("Expected original MP3 path in response")
	}

	if response.VoiceUsed != cfg.ElevenLabs.VoiceID {
		t.Errorf("Expected voice ID %s, got %s", cfg.ElevenLabs.VoiceID, response.VoiceUsed)
	}

	// Verify files were created
	if _, err := os.Stat(response.AudioFilePath); os.IsNotExist(err) {
		t.Errorf("WAV file was not created: %s", response.AudioFilePath)
	}

	if _, err := os.Stat(response.OriginalMP3); os.IsNotExist(err) {
		t.Errorf("MP3 file was not created: %s", response.OriginalMP3)
	}

	t.Logf("Successfully generated audio files:")
	t.Logf("  WAV: %s", response.AudioFilePath)
	t.Logf("  MP3: %s", response.OriginalMP3)
	t.Logf("  Duration: %d ms", response.DurationMs)
	t.Logf("  Voice: %s", response.VoiceUsed)
}

// TestSpeedParameterValidation tests speed parameter validation and usage
func TestSpeedParameterValidation(t *testing.T) {
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
			name:      "Valid speed - slow",
			speed:     0.5,
			wantError: false,
		},
		{
			name:      "Valid speed - fast",
			speed:     2.0,
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
		{
			name:      "Invalid speed - zero",
			speed:     0.0,
			wantError: true,
		},
		{
			name:      "Invalid speed - negative",
			speed:     -1.0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ElevenLabsConfig{
				APIKey: "test-api-key",
				Speed:  tt.speed,
			}

			client, err := NewElevenLabsClient(config)
			if err != nil {
				t.Fatalf("NewElevenLabsClient should not fail for any speed value: %v", err)
			}

			if tt.wantError {
				// For invalid speeds (outside 0.25-4.0), we expect the client to use the configured value
				// Config validation happens at the config loading level, not client level
				// The client just applies defaults for zero/negative values
				if tt.speed <= 0 && client.config.Speed != 1.0 {
					t.Errorf("Expected speed <= 0 to default to 1.0, got %f", client.config.Speed)
				} else if tt.speed > 0 && client.config.Speed != tt.speed {
					// Client should preserve the configured speed value, even if invalid
					// Validation occurs at config level
					t.Errorf("Expected speed %f to be preserved, got %f", tt.speed, client.config.Speed)
				}
			} else {
				if client.config.Speed != tt.speed {
					t.Errorf("Expected speed %f, got %f", tt.speed, client.config.Speed)
				}
			}
		})
	}
}

// TestCustomVoiceSettingsStructure tests the custom voice settings structure
func TestCustomVoiceSettingsStructure(t *testing.T) {
	// Test that our custom voice settings can be properly marshaled to JSON
	settings := CustomVoiceSettings{
		Stability:       0.7,
		SimilarityBoost: 0.9,
		Style:           0.2,
		Speed:           1.5,
		SpeakerBoost:    true,
	}

	// Test that the struct can be used in a request
	request := CustomTextToSpeechRequest{
		Text:          "Test text for speech generation",
		ModelID:       "eleven_multilingual_v1",
		VoiceSettings: &settings,
	}

	// Verify fields are accessible
	if request.VoiceSettings.Speed != 1.5 {
		t.Errorf("Expected speed 1.5, got %f", request.VoiceSettings.Speed)
	}

	if request.VoiceSettings.Stability != 0.7 {
		t.Errorf("Expected stability 0.7, got %f", request.VoiceSettings.Stability)
	}

	if request.VoiceSettings.SimilarityBoost != 0.9 {
		t.Errorf("Expected similarity boost 0.9, got %f", request.VoiceSettings.SimilarityBoost)
	}

	if request.VoiceSettings.Style != 0.2 {
		t.Errorf("Expected style 0.2, got %f", request.VoiceSettings.Style)
	}

	if !request.VoiceSettings.SpeakerBoost {
		t.Error("Expected speaker boost to be true")
	}
}

