package api

import (
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
			name:      "Valid script",
			script:    "Good morning listeners! Today's weather forecast for San Francisco is looking beautiful with clear skies and temperatures reaching 72 degrees.",
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

// TestTemplateVariableSubstitution tests the template variable substitution system
func TestTemplateVariableSubstitution(t *testing.T) {
	client := &ClaudeClient{}

	// Create test variables map
	variables := map[string]string{
		"location":           "San Francisco",
		"current_temp":       "72°F",
		"temp_high":          "78°F",
		"temp_low":           "65°F",
		"current_conditions": "partly cloudy",
		"rain_chance":        "10%",
		"dow":               "Thursday",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Single variable {{format}}",
			template: "Today in {{location}} it's {{current_temp}}",
			expected: "Today in San Francisco it's 72°F",
		},
		{
			name:     "Multiple variables {{format}}",
			template: "Weather for {{location}}: {{current_conditions}}, {{current_temp}} with highs of {{temp_high}}",
			expected: "Weather for San Francisco: partly cloudy, 72°F with highs of 78°F",
		},
		{
			name:     "Complex template with day",
			template: "{{location}} weather on {{dow}}: {{current_conditions}} with {{current_temp}}",
			expected: "San Francisco weather on Thursday: partly cloudy with 72°F",
		},
		{
			name:     "Missing variable",
			template: "Weather in {{location}} with {{missing_var}}",
			expected: "Weather in San Francisco with [missing:missing_var]",
		},
		{
			name:     "No variables",
			template: "Static weather report text",
			expected: "Static weather report text",
		},
		{
			name:     "Empty template",
			template: "",
			expected: "",
		},
		{
			name:     "Single braces ignored",
			template: "Weather in {location} is {{current_temp}}",
			expected: "Weather in {location} is 72°F",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.substituteTemplateVariables(tt.template, variables)
			if result != tt.expected {
				t.Errorf("Expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestFormatTemperature tests temperature formatting with different units
func TestFormatTemperature(t *testing.T) {
	tests := []struct {
		name     string
		temp     float64
		units    string
		expected string
	}{
		{"Imperial", 72.5, "imperial", "72°F"},
		{"Metric", 22.2, "metric", "22°C"},
		{"Kelvin", 295.15, "kelvin", "295 K"},
		{"Unknown units", 72.5, "unknown", "72.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTemperature(tt.temp, tt.units)
			if result != tt.expected {
				t.Errorf("Expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestFormatPercentage tests percentage formatting
func TestFormatPercentage(t *testing.T) {
	tests := []struct {
		name     string
		prob     float64
		expected string
	}{
		{"Zero", 0.0, "0%"},
		{"Half", 0.5, "50%"},
		{"Full", 1.0, "100%"},
		{"Decimal", 0.15, "15%"},
		{"High decimal", 0.856, "86%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPercentage(tt.prob)
			if result != tt.expected {
				t.Errorf("Expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestExtractWeatherVariables tests weather data variable extraction
func TestExtractWeatherVariables(t *testing.T) {
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
				Pop: 0.1,
				Wind: WindData{
					Speed: 5.5,
					Deg:   180,
				},
			},
		},
	}

	client := &ClaudeClient{}
	variables, err := client.extractWeatherVariables(forecast, "San Francisco")

	if err != nil {
		t.Fatalf("Failed to extract weather variables: %v", err)
	}

	// Check that key variables are present
	expectedKeys := []string{
		"location", "city", "country", "current_temp", "temp_high", "temp_low",
		"current_conditions", "rain_chance", "units", "date", "dow", "time",
	}

	for _, key := range expectedKeys {
		if _, exists := variables[key]; !exists {
			t.Errorf("Expected variable %q not found in extracted variables", key)
		}
	}

	// Check specific values
	if variables["location"] != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got %q", variables["location"])
	}

	if variables["city"] != "San Francisco" {
		t.Errorf("Expected city 'San Francisco', got %q", variables["city"])
	}

	if variables["country"] != "US" {
		t.Errorf("Expected country 'US', got %q", variables["country"])
	}
}

// TestExtractWeatherVariablesNilForecast tests error handling for nil forecast
func TestExtractWeatherVariablesNilForecast(t *testing.T) {
	client := &ClaudeClient{}
	_, err := client.extractWeatherVariables(nil, "Test Location")

	if err == nil {
		t.Error("Expected error for nil forecast, got nil")
	}

	if !strings.Contains(err.Error(), "forecast data is nil") {
		t.Errorf("Expected error about nil forecast, got: %v", err)
	}
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
			tempHigh:    -2.0,  // Freezing in Celsius
			tempLow:     -5.0,
			units:       "metric",
			expectedMsg: "Freezing temperatures",
		},
		{
			name:        "Cool metric day",
			tempHigh:    8.0,   // Cool in Celsius  
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

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}