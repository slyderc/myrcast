package api

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"
)

// AIDEV-NOTE: Comprehensive test suite for weather API functionality

const (
	// Test API key - replace with actual key for integration tests
	testAPIKey = "ccaedd0261a89cc75e0650b2d64dd71f"
	
	// Test coordinates (San Francisco)
	testLatitude  = 37.7749
	testLongitude = -122.4194
)

func TestNewWeatherClient(t *testing.T) {
	client := NewWeatherClient(testAPIKey)
	
	if client == nil {
		t.Fatal("Expected non-nil weather client")
	}
	
	if client.apiKey != testAPIKey {
		t.Errorf("Expected API key %s, got %s", testAPIKey, client.apiKey)
	}
	
	if client.client == nil {
		t.Fatal("Expected non-nil HTTP client")
	}
}

func TestNewWeatherClientWithRateLimit(t *testing.T) {
	client := NewWeatherClientWithRateLimit(testAPIKey)
	
	if client == nil {
		t.Fatal("Expected non-nil weather client with rate limit")
	}
	
	if client.WeatherClient == nil {
		t.Fatal("Expected embedded weather client")
	}
	
	if client.rateLimiter == nil {
		t.Fatal("Expected non-nil rate limiter")
	}
}

func TestConvertTemperature(t *testing.T) {
	tests := []struct {
		name     string
		temp     float64
		fromUnit string
		toUnit   string
		expected float64
		delta    float64
	}{
		{
			name:     "Celsius to Fahrenheit",
			temp:     0,
			fromUnit: "metric",
			toUnit:   "imperial",
			expected: 32,
			delta:    0.1,
		},
		{
			name:     "Fahrenheit to Celsius",
			temp:     32,
			fromUnit: "imperial",
			toUnit:   "metric",
			expected: 0,
			delta:    0.1,
		},
		{
			name:     "Celsius to Kelvin",
			temp:     0,
			fromUnit: "metric",
			toUnit:   "kelvin",
			expected: 273.15,
			delta:    0.1,
		},
		{
			name:     "Kelvin to Celsius",
			temp:     273.15,
			fromUnit: "kelvin",
			toUnit:   "metric",
			expected: 0,
			delta:    0.1,
		},
		{
			name:     "Same unit conversion",
			temp:     25,
			fromUnit: "metric",
			toUnit:   "metric",
			expected: 25,
			delta:    0.1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertTemperature(tt.temp, tt.fromUnit, tt.toUnit)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestConvertWindSpeed(t *testing.T) {
	tests := []struct {
		name     string
		speed    float64
		fromUnit string
		toUnit   string
		expected float64
		delta    float64
	}{
		{
			name:     "m/s to mph",
			speed:    10,
			fromUnit: "metric",
			toUnit:   "imperial",
			expected: 22.37,
			delta:    0.1,
		},
		{
			name:     "mph to m/s",
			speed:    22.37,
			fromUnit: "imperial",
			toUnit:   "metric",
			expected: 10,
			delta:    0.1,
		},
		{
			name:     "m/s to km/h",
			speed:    10,
			fromUnit: "metric",
			toUnit:   "kmh",
			expected: 36,
			delta:    0.1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertWindSpeed(tt.speed, tt.fromUnit, tt.toUnit)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestConvertPressure(t *testing.T) {
	tests := []struct {
		name     string
		pressure float64
		fromUnit string
		toUnit   string
		expected float64
		delta    float64
	}{
		{
			name:     "hPa to inHg",
			pressure: 1013.25,
			fromUnit: "hpa",
			toUnit:   "inhg",
			expected: 29.92,
			delta:    0.1,
		},
		{
			name:     "inHg to hPa",
			pressure: 29.92,
			fromUnit: "inhg",
			toUnit:   "hpa",
			expected: 1013.25,
			delta:    1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPressure(tt.pressure, tt.fromUnit, tt.toUnit)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestGetUnitSuffix(t *testing.T) {
	tests := []struct {
		measurement string
		units       string
		expected    string
	}{
		{"temperature", "imperial", "°F"},
		{"temperature", "metric", "°C"},
		{"temperature", "kelvin", "K"},
		{"wind", "imperial", "mph"},
		{"wind", "metric", "m/s"},
		{"pressure", "imperial", "inHg"},
		{"pressure", "metric", "hPa"},
		{"precipitation", "metric", "mm"},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.measurement, tt.units), func(t *testing.T) {
			result := GetUnitSuffix(tt.measurement, tt.units)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDegreesToCardinal(t *testing.T) {
	tests := []struct {
		degrees  float64
		expected string
	}{
		{0, "N"},
		{45, "NE"},
		{90, "E"},
		{135, "SE"},
		{180, "S"},
		{225, "SW"},
		{270, "W"},
		{315, "NW"},
		{360, "N"},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f_degrees", tt.degrees), func(t *testing.T) {
			result := degreesToCardinal(tt.degrees)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatWindConditions(t *testing.T) {
	tests := []struct {
		name     string
		wind     WindData
		expected string
	}{
		{
			name: "Calm wind",
			wind: WindData{Speed: 0, Deg: 0, Gust: 0},
			expected: "Calm",
		},
		{
			name: "Light wind",
			wind: WindData{Speed: 1.5, Deg: 90, Gust: 0},
			expected: "Light E winds at 1.5",
		},
		{
			name: "Wind with gusts",
			wind: WindData{Speed: 10, Deg: 270, Gust: 18},
			expected: "Moderate W winds at 10.0 (gusts to 18.0)",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWindConditions(tt.wind)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsNotableWeatherCondition(t *testing.T) {
	tests := []struct {
		condition string
		expected  bool
	}{
		{"Thunderstorm", true},
		{"Rain", true},
		{"Snow", true},
		{"Clear", false},
		{"Clouds", false},
		{"Fog", true},
		{"Tornado", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			result := isNotableWeatherCondition(tt.condition)
			if result != tt.expected {
				t.Errorf("Expected %t for %s, got %t", tt.expected, tt.condition, result)
			}
		})
	}
}

func TestValidateForecastParams(t *testing.T) {
	tests := []struct {
		name    string
		params  ForecastParams
		wantErr bool
	}{
		{
			name: "Valid parameters",
			params: ForecastParams{
				Latitude:  testLatitude,
				Longitude: testLongitude,
				Units:     "metric",
				Count:     10,
			},
			wantErr: false,
		},
		{
			name: "Invalid latitude",
			params: ForecastParams{
				Latitude:  91,
				Longitude: testLongitude,
				Units:     "metric",
			},
			wantErr: true,
		},
		{
			name: "Invalid longitude",
			params: ForecastParams{
				Latitude:  testLatitude,
				Longitude: 181,
				Units:     "metric",
			},
			wantErr: true,
		},
		{
			name: "Invalid units",
			params: ForecastParams{
				Latitude:  testLatitude,
				Longitude: testLongitude,
				Units:     "invalid",
			},
			wantErr: true,
		},
		{
			name: "Invalid count",
			params: ForecastParams{
				Latitude:  testLatitude,
				Longitude: testLongitude,
				Units:     "metric",
				Count:     50,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateForecastParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateForecastParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	// Test rate limiter with small limits for quick testing
	rl := NewRateLimiter(2, time.Second)
	ctx := context.Background()
	
	// First two requests should pass immediately
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Errorf("First request failed: %v", err)
	}
	if err := rl.Wait(ctx); err != nil {
		t.Errorf("Second request failed: %v", err)
	}
	
	// Third request should be delayed
	if err := rl.Wait(ctx); err != nil {
		t.Errorf("Third request failed: %v", err)
	}
	
	elapsed := time.Since(start)
	if elapsed < time.Second {
		t.Errorf("Expected rate limiting delay, but elapsed time was %v", elapsed)
	}
}

// Integration test - requires valid API key
func TestGetForecastIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := NewWeatherClient(testAPIKey)
	
	params := ForecastParams{
		Latitude:  testLatitude,
		Longitude: testLongitude,
		Units:     "metric",
		Count:     5,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	forecast, err := client.GetForecast(ctx, params)
	if err != nil {
		t.Fatalf("GetForecast failed: %v", err)
	}
	
	if forecast == nil {
		t.Fatal("Expected non-nil forecast")
	}
	
	if len(forecast.List) == 0 {
		t.Fatal("Expected forecast entries")
	}
	
	if forecast.City.Name == "" {
		t.Error("Expected city name in forecast")
	}
	
	// Validate first forecast entry
	entry := forecast.List[0]
	if entry.Main.Temp == 0 {
		t.Error("Expected non-zero temperature")
	}
	
	if len(entry.Weather) == 0 {
		t.Error("Expected weather conditions")
	}
	
	t.Logf("Successfully retrieved forecast for %s, %s", forecast.City.Name, forecast.City.Country)
	t.Logf("First entry: %.1f°C, %s", entry.Main.Temp, entry.Weather[0].Description)
}

// Integration test for today's weather extraction
func TestExtractTodayWeatherIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := NewWeatherClient(testAPIKey)
	
	params := ForecastParams{
		Latitude:  testLatitude,
		Longitude: testLongitude,
		Units:     "metric",
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	forecast, err := client.GetForecast(ctx, params)
	if err != nil {
		t.Fatalf("GetForecast failed: %v", err)
	}
	
	todayData, err := client.ExtractTodayWeather(forecast)
	if err != nil {
		t.Fatalf("ExtractTodayWeather failed: %v", err)
	}
	
	if todayData == nil {
		t.Fatal("Expected non-nil today weather data")
	}
	
	if todayData.Location == "" {
		t.Error("Expected location name")
	}
	
	if todayData.CurrentConditions == "" {
		t.Error("Expected current conditions")
	}
	
	if todayData.WindConditions == "" {
		t.Error("Expected wind conditions")
	}
	
	if todayData.TempHigh == 0 && todayData.TempLow == 0 {
		t.Error("Expected non-zero temperature values")
	}
	
	t.Logf("Today's weather for %s:", todayData.Location)
	t.Logf("  Temperature: %.1f°C (high) / %.1f°C (low)", todayData.TempHigh, todayData.TempLow)
	t.Logf("  Current: %.1f°C, %s", todayData.CurrentTemp, todayData.CurrentConditions)
	t.Logf("  Rain chance: %.0f%%", todayData.RainChance)
	t.Logf("  Wind: %s", todayData.WindConditions)
	if len(todayData.WeatherAlerts) > 0 {
		t.Logf("  Alerts: %v", todayData.WeatherAlerts)
	}
}

// Integration test for rate-limited client
func TestWeatherClientWithRateLimitIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := NewWeatherClientWithRateLimit(testAPIKey)
	
	params := ForecastParams{
		Latitude:  testLatitude,
		Longitude: testLongitude,
		Units:     "imperial", // Test different units
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	todayData, err := client.GetTodayWeatherWithFallback(ctx, params, "metric")
	if err != nil {
		t.Fatalf("GetTodayWeatherWithFallback failed: %v", err)
	}
	
	if todayData == nil {
		t.Fatal("Expected non-nil today weather data")
	}
	
	// Verify unit conversion worked
	if todayData.Units != "metric" {
		t.Errorf("Expected metric units after conversion, got %s", todayData.Units)
	}
	
	t.Logf("Weather with unit conversion (imperial -> metric):")
	t.Logf("  %s: %.1f°C / %.1f°C", todayData.Location, todayData.TempHigh, todayData.TempLow)
	t.Logf("  %s, %.0f%% rain chance", todayData.CurrentConditions, todayData.RainChance)
}

func TestConvertWeatherData(t *testing.T) {
	client := NewWeatherClient(testAPIKey)
	
	originalData := &TodayWeatherData{
		TempHigh:         80, // 80°F
		TempLow:          60, // 60°F
		CurrentTemp:      70, // 70°F
		CurrentConditions: "Sunny",
		RainChance:       20,
		WindConditions:   "Light NW winds at 10.0",
		WeatherAlerts:    []string{},
		LastUpdated:      time.Now(),
		Units:            "imperial",
		Location:         "Test City",
	}
	
	converted := client.ConvertWeatherData(originalData, "metric")
	
	if converted == nil {
		t.Fatal("Expected non-nil converted data")
	}
	
	if converted.Units != "metric" {
		t.Errorf("Expected metric units, got %s", converted.Units)
	}
	
	// Verify temperature conversion (80°F ≈ 26.7°C)
	if math.Abs(converted.TempHigh-26.67) > 0.1 {
		t.Errorf("Expected ~26.7°C, got %.2f°C", converted.TempHigh)
	}
	
	// Verify other fields unchanged
	if converted.CurrentConditions != originalData.CurrentConditions {
		t.Error("Current conditions should remain unchanged")
	}
	
	if converted.RainChance != originalData.RainChance {
		t.Error("Rain chance should remain unchanged")
	}
}

// Benchmark test for API performance
func BenchmarkExtractTodayWeather(b *testing.B) {
	// Create mock forecast data
	mockForecast := &ForecastResponse{
		List: make([]ForecastItem, 40),
		City: CityInfo{Name: "Test City", Country: "US"},
	}
	
	now := time.Now()
	for i := range mockForecast.List {
		mockForecast.List[i] = ForecastItem{
			Dt: now.Add(time.Duration(i) * 3 * time.Hour).Unix(),
			Main: MainWeatherData{
				Temp:    20 + float64(i%10),
				TempMin: 15 + float64(i%8),
				TempMax: 25 + float64(i%12),
			},
			Weather: []WeatherCondition{{Description: "Clear"}},
			Wind:    WindData{Speed: 5, Deg: 180},
			Pop:     0.1,
		}
	}
	
	client := NewWeatherClient("test-key")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ExtractTodayWeather(mockForecast)
		if err != nil {
			b.Fatalf("ExtractTodayWeather failed: %v", err)
		}
	}
}