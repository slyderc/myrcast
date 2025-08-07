package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestCacheManager_IsValidForToday(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.json")

	cm := NewCacheManager(cacheFile)

	// Test 1: No cache file exists
	if cm.IsValidForToday() {
		t.Error("Expected IsValidForToday to return false when cache file doesn't exist")
	}

	// Test 2: Create cache with today's date
	todayCache := &WeatherCache{
		CreatedOn:     time.Now().Format("2006-01-02"),
		CreatedAt:     time.Now().Unix(),
		Location:      "Test City",
		SchemaVersion: 1,
	}

	data, _ := toml.Marshal(todayCache)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test cache file: %v", err)
	}

	if !cm.IsValidForToday() {
		t.Error("Expected IsValidForToday to return true for cache created today")
	}

	// Test 3: Create cache with yesterday's date
	yesterday := time.Now().AddDate(0, 0, -1)
	oldCache := &WeatherCache{
		CreatedOn:     yesterday.Format("2006-01-02"),
		CreatedAt:     yesterday.Unix(),
		Location:      "Test City",
		SchemaVersion: 1,
	}

	data, _ = toml.Marshal(oldCache)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test cache file: %v", err)
	}

	if cm.IsValidForToday() {
		t.Error("Expected IsValidForToday to return false for cache created yesterday")
	}
}

func TestCacheManager_ReadWrite(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.json")

	cm := NewCacheManager(cacheFile)

	// Create test data using One Call API format
	oneCall := &OneCallResponse{
		Lat:            37.7749,
		Lon:            -122.4194,
		Timezone:       "America/Los_Angeles",
		TimezoneOffset: -28800,
		Daily: []DailyData{
			{
				Dt:      time.Now().Unix(),
				Sunrise: 1700000000,
				Sunset:  1700040000,
				Temp: DailyTemperature{
					Max: 75.0,
					Min: 60.0,
				},
			},
		},
	}

	todayData := &TodayWeatherData{
		TempHigh:    75.0,
		TempLow:     60.0,
		Location:    "San Francisco, US",
		Units:       "imperial",
		LastUpdated: time.Now(),
	}

	// Test WriteOneCall
	if err := cm.WriteOneCall(oneCall, todayData); err != nil {
		t.Fatalf("Failed to write cache: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// Test Read
	cache, err := cm.Read()
	if err != nil {
		t.Fatalf("Failed to read cache: %v", err)
	}

	// Verify data
	if cache.Location != todayData.Location {
		t.Errorf("Location mismatch: got %s, want %s", cache.Location, todayData.Location)
	}

	if cache.DailyForecast.TempHigh != todayData.TempHigh {
		t.Errorf("TempHigh mismatch: got %.1f, want %.1f", cache.DailyForecast.TempHigh, todayData.TempHigh)
	}

	if cache.DailyForecast.TempLow != todayData.TempLow {
		t.Errorf("TempLow mismatch: got %.1f, want %.1f", cache.DailyForecast.TempLow, todayData.TempLow)
	}

	if cache.SchemaVersion != 1 {
		t.Errorf("SchemaVersion mismatch: got %d, want 1", cache.SchemaVersion)
	}

	// Verify today's date
	today := time.Now().Format("2006-01-02")
	if cache.CreatedOn != today {
		t.Errorf("CreatedOn mismatch: got %s, want %s", cache.CreatedOn, today)
	}
}

func TestCacheManager_AtomicWrite(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.json")

	cm := NewCacheManager(cacheFile)

	// Create initial cache using One Call API format
	oneCall := &OneCallResponse{
		Lat:      37.7749,
		Lon:      -122.4194,
		Timezone: "Initial_City",
		Daily: []DailyData{
			{
				Temp: DailyTemperature{
					Max: 70.0,
					Min: 50.0,
				},
			},
		},
	}
	todayData := &TodayWeatherData{
		TempHigh: 70.0,
		Location: "Initial City",
	}

	if err := cm.WriteOneCall(oneCall, todayData); err != nil {
		t.Fatalf("Failed to write initial cache: %v", err)
	}

	// Read initial data
	cache1, _ := cm.Read()

	// Write new data
	oneCall.Timezone = "Updated_City"
	oneCall.Daily[0].Temp.Max = 80.0
	todayData.Location = "Updated City"
	todayData.TempHigh = 80.0

	if err := cm.WriteOneCall(oneCall, todayData); err != nil {
		t.Fatalf("Failed to write updated cache: %v", err)
	}

	// Verify temp file doesn't exist (atomic write cleanup)
	tempFile := cacheFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Temporary file was not cleaned up after write")
	}

	// Read updated data
	cache2, _ := cm.Read()

	// Verify update
	if cache2.Location == cache1.Location {
		t.Error("Cache was not updated")
	}

	if cache2.DailyForecast.TempHigh != 80.0 {
		t.Errorf("TempHigh not updated: got %.1f, want 80.0", cache2.DailyForecast.TempHigh)
	}
}

func TestCacheManager_CorruptedCache(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.json")

	cm := NewCacheManager(cacheFile)

	// Write corrupted TOML
	if err := os.WriteFile(cacheFile, []byte("[invalid toml"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted cache file: %v", err)
	}

	// Try to read
	_, err := cm.Read()
	if err == nil {
		t.Error("Expected error when reading corrupted cache")
	}

	// Verify IsValidForToday returns false
	if cm.IsValidForToday() {
		t.Error("Expected IsValidForToday to return false for corrupted cache")
	}
}

func TestCacheManager_Delete(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.json")

	cm := NewCacheManager(cacheFile)

	// Create a cache file
	if err := os.WriteFile(cacheFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete it
	if err := cm.Delete(); err != nil {
		t.Fatalf("Failed to delete cache: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("Cache file still exists after delete")
	}

	// Delete non-existent file should not error
	if err := cm.Delete(); err != nil {
		t.Error("Delete non-existent file should not return error")
	}
}
