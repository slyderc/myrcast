package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"myrcast/api"
	"myrcast/internal/logger"
)

func main() {
	// Set up console logging
	logger.Info("Starting weather API demonstration")
	
	// Create weather client with rate limiting
	client := api.NewWeatherClientWithRateLimit("ccaedd0261a89cc75e0650b2d64dd71f")
	
	// Demo coordinates for different cities
	cities := []struct {
		name string
		lat  float64
		lon  float64
	}{
		{"San Francisco, CA", 37.7749, -122.4194},
		{"New York, NY", 40.7128, -74.0060},
		{"Miami, FL", 25.7617, -80.1918},
		{"Seattle, WA", 47.6062, -122.3321},
	}
	
	for _, city := range cities {
		fmt.Printf("\n=== Weather for %s ===\n", city.name)
		
		params := api.ForecastParams{
			Latitude:  city.lat,
			Longitude: city.lon,
			Units:     "imperial",
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		
		// Get today's weather with unit conversion to metric
		weatherData, err := client.GetTodayWeatherWithFallback(ctx, params, "metric")
		cancel()
		
		if err != nil {
			log.Printf("Error fetching weather for %s: %v", city.name, err)
			continue
		}
		
		// Display results
		fmt.Printf("Location: %s\n", weatherData.Location)
		fmt.Printf("Current: %.1f°C, %s\n", weatherData.CurrentTemp, weatherData.CurrentConditions)
		fmt.Printf("High/Low: %.1f°C / %.1f°C\n", weatherData.TempHigh, weatherData.TempLow)
		fmt.Printf("Rain chance: %.0f%%\n", weatherData.RainChance)
		fmt.Printf("Wind: %s\n", weatherData.WindConditions)
		
		if len(weatherData.WeatherAlerts) > 0 {
			fmt.Printf("Alerts: %v\n", weatherData.WeatherAlerts)
		}
		
		fmt.Printf("Last updated: %s\n", weatherData.LastUpdated.Format("3:04 PM MST"))
		
		// Demonstrate unit conversion
		fmt.Printf("\n--- Unit Conversion Demo ---\n")
		imperialData := client.ConvertWeatherData(weatherData, "imperial")
		fmt.Printf("Same data in Fahrenheit: %.1f°F / %.1f°F (current: %.1f°F)\n", 
			imperialData.TempHigh, imperialData.TempLow, imperialData.CurrentTemp)
		
		// Show unit suffixes
		fmt.Printf("Temperature unit: %s\n", api.GetUnitSuffix("temperature", weatherData.Units))
		fmt.Printf("Wind unit: %s\n", api.GetUnitSuffix("wind", weatherData.Units))
		
		// Small delay between cities to be nice to the API
		time.Sleep(1 * time.Second)
	}
	
	fmt.Printf("\n=== API Performance Demo ===\n")
	demonstrateConversions()
	
	logger.Info("Weather API demonstration completed successfully")
}

func demonstrateConversions() {
	fmt.Printf("Temperature conversions:\n")
	fmt.Printf("  0°C = %.1f°F = %.1fK\n", 
		api.ConvertTemperature(0, "metric", "imperial"),
		api.ConvertTemperature(0, "metric", "kelvin"))
	
	fmt.Printf("  100°F = %.1f°C = %.1fK\n",
		api.ConvertTemperature(100, "imperial", "metric"),
		api.ConvertTemperature(100, "imperial", "kelvin"))
	
	fmt.Printf("\nWind speed conversions:\n")
	fmt.Printf("  10 m/s = %.1f mph = %.1f km/h\n",
		api.ConvertWindSpeed(10, "metric", "imperial"),
		api.ConvertWindSpeed(10, "metric", "kmh"))
	
	fmt.Printf("  25 mph = %.1f m/s = %.1f km/h\n",
		api.ConvertWindSpeed(25, "imperial", "metric"),
		api.ConvertWindSpeed(25, "imperial", "kmh"))
	
	fmt.Printf("\nPressure conversions:\n")
	fmt.Printf("  1013.25 hPa = %.2f inHg = %.1f mmHg\n",
		api.ConvertPressure(1013.25, "hpa", "inhg"),
		api.ConvertPressure(1013.25, "hpa", "mmhg"))
}