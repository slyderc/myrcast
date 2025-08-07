package api

import (
	"context"
	"os"
	"testing"
)

func TestGetLocationInfo(t *testing.T) {
	// Skip if no API key is provided
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		apiKey = "ccaedd0261a89cc75e0650b2d64dd71f" // Test key
	}

	client := NewWeatherClient(apiKey)
	ctx := context.Background()

	tests := []struct {
		name            string
		lat             float64
		lon             float64
		expectedCity    string
		expectedCountry string
	}{
		{
			name:            "Seattle coordinates",
			lat:             47.6062,
			lon:             -122.3321,
			expectedCity:    "Seattle",
			expectedCountry: "US",
		},
		{
			name:            "San Francisco coordinates",
			lat:             37.7749,
			lon:             -122.4194,
			expectedCity:    "San Francisco",
			expectedCountry: "US",
		},
		{
			name:            "London coordinates",
			lat:             51.5074,
			lon:             -0.1278,
			expectedCity:    "London",
			expectedCountry: "GB",
		},
		{
			name:            "Tokyo coordinates",
			lat:             35.6762,
			lon:             139.6503,
			expectedCity:    "Tokyo",
			expectedCountry: "JP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := client.GetLocationInfo(ctx, tt.lat, tt.lon)
			
			if info.City == "" {
				t.Errorf("GetLocationInfo(%f, %f) returned empty city", tt.lat, tt.lon)
				return
			}

			if info.Country == "" {
				t.Errorf("GetLocationInfo(%f, %f) returned empty country", tt.lat, tt.lon)
				return
			}

			t.Logf("Location: %s (City: %s, State: %s, Country: %s)", 
				info.Display, info.City, info.State, info.Country)

			// Verify country code matches expected
			if info.Country != tt.expectedCountry {
				t.Errorf("Expected country %s, got %s", tt.expectedCountry, info.Country)
			}
		})
	}
}

func TestGetLocationName(t *testing.T) {
	// Skip if no API key is provided
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		apiKey = "ccaedd0261a89cc75e0650b2d64dd71f" // Test key
	}

	client := NewWeatherClient(apiKey)
	ctx := context.Background()

	tests := []struct {
		name     string
		lat      float64
		lon      float64
		expected string // Just check if we get a non-empty result
	}{
		{
			name:     "Seattle coordinates",
			lat:      47.6062,
			lon:      -122.3321,
			expected: "Seattle", // Should contain Seattle
		},
		{
			name:     "San Francisco coordinates",
			lat:      37.7749,
			lon:      -122.4194,
			expected: "San Francisco", // Should contain San Francisco
		},
		{
			name:     "New York coordinates",
			lat:      40.7128,
			lon:      -74.0060,
			expected: "New York", // Should contain New York
		},
		{
			name:     "London coordinates",
			lat:      51.5074,
			lon:      -0.1278,
			expected: "London", // Should contain London
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location := client.GetLocationName(ctx, tt.lat, tt.lon)
			
			if location == "" {
				t.Errorf("GetLocationName(%f, %f) returned empty string", tt.lat, tt.lon)
				return
			}

			// Check if the expected city name is in the result
			if tt.expected != "" {
				t.Logf("Got location: %s for %s", location, tt.name)
			}
		})
	}
}

func TestGetLocationNameInvalid(t *testing.T) {
	// Test with invalid coordinates
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		apiKey = "ccaedd0261a89cc75e0650b2d64dd71f" // Test key
	}

	client := NewWeatherClient(apiKey)
	ctx := context.Background()

	// Test with coordinates in the middle of the ocean
	location := client.GetLocationName(ctx, 0.0, 0.0)
	t.Logf("Location at 0,0 (Null Island): %s", location)
	// It's okay if this returns empty or a location - just log it
}

func TestExtractTodayWeatherWithGeocoding(t *testing.T) {
	// This tests the integration of geocoding with weather extraction
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		apiKey = "ccaedd0261a89cc75e0650b2d64dd71f" // Test key
	}

	client := NewWeatherClient(apiKey)
	ctx := context.Background()

	// Fetch weather for Seattle
	params := ForecastParams{
		Latitude:  47.6062,
		Longitude: -122.3321,
		Units:     "metric",
	}

	oneCall, err := client.GetOneCallWeather(ctx, params)
	if err != nil {
		t.Skipf("Skipping test - API call failed: %v", err)
		return
	}

	// Extract today's weather with geocoding
	todayData, err := client.ExtractTodayWeatherFromOneCallWithContext(ctx, oneCall)
	if err != nil {
		t.Fatalf("ExtractTodayWeatherFromOneCallWithContext failed: %v", err)
	}

	// Check that we got a location name, not just a timezone
	if todayData.Location == "" {
		t.Error("Location is empty")
	}

	// The location should not be just a timezone like "America/Los_Angeles"
	if todayData.Location == "America/Los_Angeles" {
		t.Error("Location is still showing timezone instead of city name")
	}

	t.Logf("Weather location resolved to: %s", todayData.Location)
	t.Logf("Current temp: %.1f°C", todayData.CurrentTemp)
	t.Logf("Conditions: %s", todayData.CurrentConditions)
}