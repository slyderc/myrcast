package api

import (
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
	"myrcast/internal/logger"
)

// WeatherCache represents the cached weather data structure
type WeatherCache struct {
	// Metadata
	CreatedOn   string  `toml:"created_on"`   // Date in YYYY-MM-DD format (local time)
	CreatedAt   int64   `toml:"created_at"`   // Unix timestamp for debugging
	Location    string  `toml:"location"`     // Location name (city, country)
	Latitude    float64 `toml:"latitude"`     // Location latitude
	Longitude   float64 `toml:"longitude"`    // Location longitude
	Units       string  `toml:"units"`        // Units system (metric/imperial)
	
	// Cached forecast data (daily values that don't change throughout the day)
	DailyForecast DailyCachedData `toml:"daily_forecast"`
	
	// Version for future schema changes
	SchemaVersion int `toml:"schema_version"`
}

// DailyCachedData contains forecast data that remains constant throughout the day
type DailyCachedData struct {
	TempHigh    float64 `toml:"temp_high"`    // Daily high temperature
	TempLow     float64 `toml:"temp_low"`     // Daily low temperature
	Sunrise     int64   `toml:"sunrise"`      // Sunrise time (Unix timestamp)
	Sunset      int64   `toml:"sunset"`       // Sunset time (Unix timestamp)
	CityName    string  `toml:"city_name"`    // City name from API
	Country     string  `toml:"country"`      // Country code
	Timezone    int     `toml:"timezone"`     // Timezone offset in seconds from UTC
}

// CurrentWeatherData represents live data that must be fetched fresh
type CurrentWeatherData struct {
	CurrentTemp       float64  `toml:"current_temp"`       // Current temperature
	CurrentConditions string   `toml:"current_conditions"` // Current weather description
	RainChance        float64  `toml:"rain_chance"`        // Current precipitation probability
	WindSpeed         float64  `toml:"wind_speed"`         // Current wind speed
	WindDirection     float64  `toml:"wind_direction"`     // Current wind direction in degrees
	Pressure          float64  `toml:"pressure"`           // Current atmospheric pressure (hPa)
	Humidity          int      `toml:"humidity"`           // Current humidity percentage
	WeatherAlerts     []string `toml:"weather_alerts"`     // Current weather alerts
}

// CacheManager handles weather cache operations
type CacheManager struct {
	filePath string
}

// NewCacheManager creates a new cache manager instance
func NewCacheManager(filePath string) *CacheManager {
	return &CacheManager{
		filePath: filePath,
	}
}

// IsValidForToday checks if the cache is valid for the current day
func (cm *CacheManager) IsValidForToday() bool {
	// Read existing cache
	cache, err := cm.Read()
	if err != nil {
		logger.Debug("Cache not valid: %v", err)
		return false
	}
	
	// Get current date in local time
	now := time.Now()
	today := now.Format("2006-01-02")
	
	// Check if cache was created today
	isValid := cache.CreatedOn == today
	
	logger.Debug("Cache validity check: created=%s, today=%s, valid=%v", 
		cache.CreatedOn, today, isValid)
	
	return isValid
}

// Read loads and validates the cache file
func (cm *CacheManager) Read() (*WeatherCache, error) {
	complete := logger.LogOperationStart("cache_read", map[string]any{
		"file_path": cm.filePath,
	})
	
	// Check if file exists
	if _, err := os.Stat(cm.filePath); os.IsNotExist(err) {
		complete(fmt.Errorf("cache file does not exist"))
		return nil, fmt.Errorf("cache file does not exist: %s", cm.filePath)
	}
	
	// Read file content
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		complete(fmt.Errorf("failed to read cache file: %w", err))
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}
	
	// Unmarshal TOML
	var cache WeatherCache
	if err := toml.Unmarshal(data, &cache); err != nil {
		complete(fmt.Errorf("failed to parse cache TOML: %w", err))
		return nil, fmt.Errorf("failed to parse cache TOML: %w", err)
	}
	
	// Validate schema version
	if cache.SchemaVersion != 1 {
		complete(fmt.Errorf("unsupported cache schema version: %d", cache.SchemaVersion))
		return nil, fmt.Errorf("unsupported cache schema version: %d", cache.SchemaVersion)
	}
	
	complete(nil)
	logger.Debug("Cache loaded successfully: created=%s, location=%s", 
		cache.CreatedOn, cache.Location)
	
	return &cache, nil
}

// Write saves weather data to the cache file
func (cm *CacheManager) Write(forecast *ForecastResponse, todayData *TodayWeatherData) error {
	complete := logger.LogOperationStart("cache_write", map[string]any{
		"file_path": cm.filePath,
		"location":  todayData.Location,
	})
	
	// Create cache structure
	now := time.Now()
	cache := WeatherCache{
		CreatedOn:     now.Format("2006-01-02"), // Local date
		CreatedAt:     now.Unix(),
		Location:      todayData.Location,
		Latitude:      forecast.City.Coord.Lat,
		Longitude:     forecast.City.Coord.Lon,
		Units:         todayData.Units,
		SchemaVersion: 1,
		DailyForecast: DailyCachedData{
			TempHigh: todayData.TempHigh,
			TempLow:  todayData.TempLow,
			CityName: forecast.City.Name,
			Country:  forecast.City.Country,
			Timezone: forecast.City.Timezone,
			Sunrise:  forecast.City.Sunrise,
			Sunset:   forecast.City.Sunset,
		},
	}
	
	
	// Marshal to TOML
	data, err := toml.Marshal(cache)
	if err != nil {
		complete(fmt.Errorf("failed to marshal cache: %w", err))
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}
	
	// Write to temporary file first (atomic write)
	tempFile := cm.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		complete(fmt.Errorf("failed to write temp file: %w", err))
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	
	// Rename temp file to actual cache file (atomic operation)
	if err := os.Rename(tempFile, cm.filePath); err != nil {
		// Clean up temp file if rename fails
		os.Remove(tempFile)
		complete(fmt.Errorf("failed to rename temp file: %w", err))
		return fmt.Errorf("failed to finalize cache file: %w", err)
	}
	
	complete(nil)
	logger.Debug("Weather cache saved: created=%s, location=%s, high=%.1f, low=%.1f", 
		cache.CreatedOn, cache.Location, cache.DailyForecast.TempHigh, cache.DailyForecast.TempLow)
	
	return nil
}

// Delete removes the cache file (used for testing or manual cache clearing)
func (cm *CacheManager) Delete() error {
	if err := os.Remove(cm.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}
	logger.Debug("Cache file deleted: %s", cm.filePath)
	return nil
}