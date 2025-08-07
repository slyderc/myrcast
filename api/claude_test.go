package api

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"myrcast/config"
)

// TestClaudeClientCreation tests creating a Claude client with valid configuration
func TestClaudeClientCreation(t *testing.T) {
	// Load dev.toml configuration
	cfg, err := config.LoadConfig("../dev.toml")
	if err != nil {
		t.Fatalf("Failed to load dev.toml: %v", err)
	}

	// Create Claude client with config values
	claudeConfig := ClaudeConfig{
		APIKey:      cfg.APIs.Anthropic,
		Model:       cfg.Claude.Model,
		MaxTokens:   cfg.Claude.MaxTokens,
		Temperature: cfg.Claude.Temperature,
		Timeout:     30 * time.Second,
		MaxRetries:  cfg.Claude.MaxRetries,
		BaseDelay:   time.Duration(cfg.Claude.BaseDelayMs) * time.Millisecond,
		MaxDelay:    time.Duration(cfg.Claude.MaxDelayMs) * time.Millisecond,
		RateLimit:   cfg.Claude.RateLimit,
	}

	client, err := NewClaudeClient(claudeConfig)
	if err != nil {
		t.Fatalf("Failed to create Claude client: %v", err)
	}

	// Verify client configuration
	if client.config.APIKey != cfg.APIs.Anthropic {
		t.Errorf("Expected API key from config, got different value")
	}

	if client.config.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Expected model 'claude-sonnet-4-20250514', got '%s'", client.config.Model)
	}

	if client.config.MaxTokens != 1000 {
		t.Errorf("Expected max tokens 1000, got %d", client.config.MaxTokens)
	}

	if client.config.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", client.config.Temperature)
	}
}

// TestClaudeClientDefaults tests default values when config is empty
func TestClaudeClientDefaults(t *testing.T) {
	claudeConfig := ClaudeConfig{
		APIKey: "test-api-key",
		// Leave other fields empty to test defaults
	}

	client, err := NewClaudeClient(claudeConfig)
	if err != nil {
		t.Fatalf("Failed to create Claude client: %v", err)
	}

	// Check defaults
	if client.config.Model != defaultModel {
		t.Errorf("Expected default model '%s', got '%s'", defaultModel, client.config.Model)
	}

	if client.config.MaxTokens != defaultMaxTokens {
		t.Errorf("Expected default max tokens %d, got %d", defaultMaxTokens, client.config.MaxTokens)
	}

	if client.config.Temperature != defaultTemperature {
		t.Errorf("Expected default temperature %f, got %f", defaultTemperature, client.config.Temperature)
	}

	if client.config.Timeout != defaultClaudeTimeout {
		t.Errorf("Expected default timeout %v, got %v", defaultClaudeTimeout, client.config.Timeout)
	}
}

// TestClaudeClientAPIKeyValidation tests API key validation
func TestClaudeClientAPIKeyValidation(t *testing.T) {
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
			apiKey:    "sk-ant-api-key",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claudeConfig := ClaudeConfig{
				APIKey: tt.apiKey,
			}

			_, err := NewClaudeClient(claudeConfig)
			if (err != nil) != tt.wantError {
				t.Errorf("NewClaudeClient() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestValidateGeneratedScript tests script validation logic
func TestValidateGeneratedScript(t *testing.T) {
	client := &ClaudeClient{}

	tests := []struct {
		name      string
		script    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Empty script",
			script:    "",
			wantError: true,
			errorMsg:  "empty",
		},
		{
			name:      "Whitespace only script",
			script:    "   \n\t   ",
			wantError: true,
			errorMsg:  "empty",
		},
		{
			name:      "Too short script",
			script:    "Hello",
			wantError: true,
			errorMsg:  "too short",
		},
		{
			name:      "Too long script",
			script:    string(make([]byte, 5001)),
			wantError: true,
			errorMsg:  "too long",
		},
		{
			name:      "Too few words",
			script:    "This script has only ten words in total content here.",
			wantError: true,
			errorMsg:  "too few words",
		},
		{
			name:      "Too many words",
			script:    strings.Repeat("word ", 801) + "weather",
			wantError: true,
			errorMsg:  "too many words",
		},
		{
			name:      "No weather content",
			script:    "This is a very long script with many words but it does not contain any information about meteorological conditions or atmospheric phenomena that would be relevant for a broadcast.",
			wantError: true,
			errorMsg:  "weather-related content",
		},
		{
			name:      "Valid weather script",
			script:    "Good morning listeners! Today's weather forecast for San Francisco is looking beautiful with clear skies and temperatures reaching 72 degrees. Light winds from the west at 5 mph.",
			wantError: false,
		},
		{
			name:      "Valid script with different weather terms",
			script:    "The forecast shows cloudy skies this morning with a chance of rain later. Temperatures will be in the mid-60s with moderate wind conditions throughout the day.",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateGeneratedScript(tt.script)
			if (err != nil) != tt.wantError {
				t.Errorf("validateGeneratedScript() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" {
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// TestClaudeAPIIntegration tests actual API call with dev.toml credentials
func TestClaudeAPIIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Load dev.toml configuration
	cfg, err := config.LoadConfig("../dev.toml")
	if err != nil {
		t.Fatalf("Failed to load dev.toml: %v", err)
	}

	// Create Claude client
	claudeConfig := ClaudeConfig{
		APIKey:      cfg.APIs.Anthropic,
		Model:       cfg.Claude.Model,
		MaxTokens:   cfg.Claude.MaxTokens,
		Temperature: cfg.Claude.Temperature,
		Timeout:     30 * time.Second,
		MaxRetries:  cfg.Claude.MaxRetries,
		BaseDelay:   time.Duration(cfg.Claude.BaseDelayMs) * time.Millisecond,
		MaxDelay:    time.Duration(cfg.Claude.MaxDelayMs) * time.Millisecond,
		RateLimit:   cfg.Claude.RateLimit,
	}

	client, err := NewClaudeClient(claudeConfig)
	if err != nil {
		t.Fatalf("Failed to create Claude client: %v", err)
	}

	// Test that the client is properly initialized
	if client.config.APIKey != cfg.APIs.Anthropic {
		t.Error("Claude client API key not properly set")
	}

	// We can't test GenerateWeatherReport yet since formatWeatherContext is not implemented
	// But we've verified the client is created properly with the correct configuration
}

// TestFormatWeatherContext tests the weather context formatting for Claude
func TestFormatWeatherContext(t *testing.T) {
	// Create mock forecast data
	forecast := &ForecastResponse{
		City: CityInfo{
			Name:    "San Francisco",
			Country: "US",
		},
		List: []ForecastItem{
			{
				Dt: time.Now().Unix(),
				Main: MainWeatherData{
					Temp:    72.5,
					TempMin: 65.0,
					TempMax: 78.0,
				},
				Weather: []WeatherCondition{
					{
						Main:        "Clouds",
						Description: "partly cloudy",
					},
				},
				Pop: 0.2, // 20% chance of rain
				Wind: WindData{
					Speed: 5.5,
					Deg:   180,
				},
			},
		},
	}

	client := &ClaudeClient{}
	context, err := client.formatWeatherContext(forecast, "San Francisco, CA")

	if err != nil {
		t.Fatalf("Failed to format weather context: %v", err)
	}

	// Check that context contains expected sections
	expectedSections := []string{
		"WEATHER DATA FOR SAN FRANCISCO, CA",
		"CURRENT CONDITIONS:",
		"TODAY'S FORECAST:",
		"BROADCAST NOTES:",
		"Temperature:",
		"Conditions:",
		"Wind:",
		"High temperature:",
		"Low temperature:",
		"Precipitation chance:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(context, section) {
			t.Errorf("Expected context to contain %q, but it was missing", section)
		}
	}

	// Check that context includes radio guidance
	if !strings.Contains(context, "radio-friendly tone") {
		t.Error("Expected context to include radio guidance")
	}

	// Check that rain guidance is appropriate for 20% chance
	if !strings.Contains(context, "Rain is unlikely") {
		t.Errorf("Expected context to indicate rain is unlikely for 20%% chance. Context:\n%s", context)
	}

	// Verify context is not empty and has reasonable length
	if len(context) < 200 {
		t.Errorf("Context seems too short: %d characters", len(context))
	}

	if len(context) > 2000 {
		t.Errorf("Context seems too long: %d characters", len(context))
	}
}

// TestFormatWeatherContextWithAlerts tests context formatting with weather alerts
func TestFormatWeatherContextWithAlerts(t *testing.T) {
	// Create forecast with severe weather
	forecast := &ForecastResponse{
		City: CityInfo{
			Name:    "Miami",
			Country: "US",
		},
		List: []ForecastItem{
			{
				Dt: time.Now().Unix(),
				Main: MainWeatherData{
					Temp:    85.0,
					TempMin: 78.0,
					TempMax: 92.0,
				},
				Weather: []WeatherCondition{
					{
						Main:        "Thunderstorm",
						Description: "thunderstorms with heavy rain",
					},
				},
				Pop: 0.9, // 90% chance of rain
				Wind: WindData{
					Speed: 15.0,
					Deg:   90,
				},
			},
		},
	}

	client := &ClaudeClient{}
	context, err := client.formatWeatherContext(forecast, "Miami, FL")

	if err != nil {
		t.Fatalf("Failed to format weather context: %v", err)
	}

	// Check for hot weather guidance
	if !strings.Contains(context, "Hot day") {
		t.Error("Expected context to include hot weather guidance for 92°F high")
	}

	// Check for high rain chance guidance
	if !strings.Contains(context, "Rain is likely") {
		t.Error("Expected context to indicate rain is likely for 90% chance")
	}
}

// TestFormatWeatherContextNilWeather tests error handling for nil weather data
func TestFormatWeatherContextNilWeather(t *testing.T) {
	client := &ClaudeClient{}
	_, err := client.formatWeatherContext(nil, "Test Location")

	if err == nil {
		t.Error("Expected error for nil weather data, got nil")
	}

	if !strings.Contains(err.Error(), "weather data is nil") {
		t.Errorf("Expected error about nil weather data, got: %v", err)
	}
}

// TestFormatWeatherContextTemperatureGuidance tests temperature-specific guidance
func TestFormatWeatherContextTemperatureGuidance(t *testing.T) {
	tests := []struct {
		name        string
		tempHigh    float64
		tempLow     float64
		units       string
		expectedMsg string
	}{
		{
			name:        "Hot imperial day",
			tempHigh:    95.0,
			tempLow:     75.0,
			units:       "imperial",
			expectedMsg: "Hot day",
		},
		{
			name:        "Freezing metric day",
			tempHigh:    -2.0, // Freezing in Celsius
			tempLow:     -5.0,
			units:       "metric",
			expectedMsg: "Freezing temperatures",
		},
		{
			name:        "Cool metric day",
			tempHigh:    8.0, // Cool in Celsius
			tempLow:     2.0,
			units:       "metric",
			expectedMsg: "Cool day",
		},
		{
			name:        "Hot metric day",
			tempHigh:    35.0,
			tempLow:     25.0,
			units:       "metric",
			expectedMsg: "Hot day",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forecast := &ForecastResponse{
				City: CityInfo{Name: "Test City", Country: "US"},
				List: []ForecastItem{
					{
						Dt: time.Now().Unix(),
						Main: MainWeatherData{
							Temp:    tt.tempHigh,
							TempMin: tt.tempLow,
							TempMax: tt.tempHigh,
						},
						Weather: []WeatherCondition{{Main: "Clear", Description: "clear sky"}},
						Pop:     0.0,
						Wind:    WindData{Speed: 5.0, Deg: 180},
					},
				},
			}

			client := &ClaudeClient{}
			context, err := client.formatWeatherContext(forecast, "Test Location")

			if err != nil {
				t.Fatalf("Failed to format weather context: %v", err)
			}

			if !strings.Contains(context, tt.expectedMsg) {
				t.Errorf("Expected context to contain %q for %s", tt.expectedMsg, tt.name)
			}
		})
	}
}

// TestClaudeRateLimiter tests the rate limiting functionality
func TestClaudeRateLimiter(t *testing.T) {
	// Create rate limiter allowing 2 requests per minute
	limiter := NewClaudeRateLimiter(2)

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

// TestClaudeRateLimiterCancellation tests context cancellation
func TestClaudeRateLimiterCancellation(t *testing.T) {
	limiter := NewClaudeRateLimiter(1)

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

// TestParseClaudeError tests error parsing and retry logic
func TestParseClaudeError(t *testing.T) {
	client := &ClaudeClient{}

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
			claudeErr := client.parseClaudeError(tt.err)

			if claudeErr.Type != tt.expectType {
				t.Errorf("Expected error type %q, got %q", tt.expectType, claudeErr.Type)
			}

			if claudeErr.IsRetryable() != tt.retryable {
				t.Errorf("Expected retryable %v, got %v", tt.retryable, claudeErr.IsRetryable())
			}
		})
	}
}

// TestCalculateRetryDelay tests exponential backoff calculation
func TestCalculateRetryDelay(t *testing.T) {
	client := &ClaudeClient{
		config: ClaudeConfig{
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

// TestCalculateRetryDelayWithJitter tests jitter functionality
func TestCalculateRetryDelayWithJitter(t *testing.T) {
	client := &ClaudeClient{
		config: ClaudeConfig{
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

// TestClaudeClientConfigDefaults tests that retry defaults are properly applied
func TestClaudeClientConfigDefaults(t *testing.T) {
	config := ClaudeConfig{
		APIKey: "test-key",
		// Leave retry config empty to test defaults
	}

	client, err := NewClaudeClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that defaults were applied
	if client.config.MaxRetries != defaultMaxRetries {
		t.Errorf("Expected MaxRetries %d, got %d", defaultMaxRetries, client.config.MaxRetries)
	}

	if client.config.BaseDelay != defaultBaseDelay {
		t.Errorf("Expected BaseDelay %v, got %v", defaultBaseDelay, client.config.BaseDelay)
	}

	if client.config.MaxDelay != defaultMaxDelay {
		t.Errorf("Expected MaxDelay %v, got %v", defaultMaxDelay, client.config.MaxDelay)
	}

	if client.config.RateLimit != defaultRateLimit {
		t.Errorf("Expected RateLimit %d, got %d", defaultRateLimit, client.config.RateLimit)
	}

	// Check that rate limiter was created
	if client.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
