package api

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"myrcast/internal/logger"
)

const (
	// OpenWeather API base URL and endpoints
	openWeatherBaseURL = "https://api.openweathermap.org/data/2.5"
	geocodingBaseURL   = "https://api.openweathermap.org/geo/1.0"
	forecastEndpoint   = "/forecast"
	weatherEndpoint    = "/weather"
	reverseGeoEndpoint = "/reverse"

	// Default timeout for API requests
	defaultTimeout = 10 * time.Second

	// User-Agent for API requests
	userAgent = "Myrcast/1.0"
)

// WeatherClient handles OpenWeather API interactions
type WeatherClient struct {
	client *resty.Client
	apiKey string
}

// NewWeatherClient creates a new OpenWeather API client with authentication
func NewWeatherClient(apiKey string) *WeatherClient {
	// AIDEV-NOTE: Using go-resty for cleaner HTTP client setup with built-in JSON handling
	client := resty.New().
		SetBaseURL(openWeatherBaseURL).
		SetHeader("User-Agent", userAgent).
		SetTimeout(defaultTimeout).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	// Add debug logging for development
	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		// Convert http.Header to map[string]string for logging
		headers := make(map[string]string)
		for key, values := range req.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		logger.LogAPIRequest(req.Method, req.URL, headers)
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		duration := resp.Time().String()
		bodySize := len(resp.Body())
		logger.LogAPIResponse(resp.Request.Method, resp.Request.URL, resp.StatusCode(), duration, bodySize)
		return nil
	})

	return &WeatherClient{
		client: client,
		apiKey: apiKey,
	}
}

// SetTimeout configures the HTTP client timeout
func (w *WeatherClient) SetTimeout(timeout time.Duration) {
	w.client.SetTimeout(timeout)
}

// SetRetryPolicy configures retry behavior for transient failures
func (w *WeatherClient) SetRetryPolicy(retryCount int, waitTime, maxWaitTime time.Duration) {
	w.client.SetRetryCount(retryCount).
		SetRetryWaitTime(waitTime).
		SetRetryMaxWaitTime(maxWaitTime)
}

// ForecastParams contains parameters for weather forecast requests
type ForecastParams struct {
	Latitude  float64 // Latitude coordinate
	Longitude float64 // Longitude coordinate
	Units     string  // Units: metric, imperial, or kelvin
	Count     int     // Number of forecast entries (optional, max 40)
}

// CurrentWeatherResponse represents the OpenWeather current weather API response
type CurrentWeatherResponse struct {
	Coord      Coordinates        `json:"coord"`          // Coordinates
	Weather    []WeatherCondition `json:"weather"`        // Weather conditions array
	Base       string             `json:"base"`           // Internal parameter
	Main       MainWeatherData    `json:"main"`           // Temperature, pressure, humidity data
	Visibility int                `json:"visibility"`     // Visibility in meters
	Wind       WindData           `json:"wind"`           // Wind data
	Clouds     CloudData          `json:"clouds"`         // Cloud coverage data
	Rain       *PrecipitationData `json:"rain,omitempty"` // Rain data (if present)
	Snow       *PrecipitationData `json:"snow,omitempty"` // Snow data (if present)
	Dt         int64              `json:"dt"`             // Time of data calculation (Unix timestamp)
	Sys        struct {
		Type    int    `json:"type"`    // Internal parameter
		Id      int    `json:"id"`      // Internal parameter
		Country string `json:"country"` // Country code
		Sunrise int64  `json:"sunrise"` // Sunrise time (Unix timestamp)
		Sunset  int64  `json:"sunset"`  // Sunset time (Unix timestamp)
	} `json:"sys"`
	Timezone int    `json:"timezone"` // Shift in seconds from UTC
	Id       int    `json:"id"`       // City ID
	Name     string `json:"name"`     // City name
	Cod      int    `json:"cod"`      // Internal parameter
}

// GetCurrentWeather fetches current weather data from OpenWeather API
func (w *WeatherClient) GetCurrentWeather(ctx context.Context, params ForecastParams) (*CurrentWeatherResponse, error) {
	complete := logger.LogOperationStart("weather_api_current", map[string]any{
		"endpoint":  "weather",
		"latitude":  params.Latitude,
		"longitude": params.Longitude,
		"units":     params.Units,
	})

	// Build query parameters for the API request
	queryParams := map[string]interface{}{
		"lat":   params.Latitude,
		"lon":   params.Longitude,
		"appid": w.apiKey,
		"units": params.Units,
	}

	var weatherResp CurrentWeatherResponse

	// Execute the HTTP request with context for cancellation
	resp, err := w.client.R().
		SetContext(ctx).
		SetQueryParams(convertToStringMap(queryParams)).
		SetResult(&weatherResp).
		Get(weatherEndpoint)

	if err != nil {
		complete(fmt.Errorf("HTTP request failed: %w", err))
		return nil, fmt.Errorf("failed to fetch current weather: %w", err)
	}

	// Check for HTTP error status codes
	if !resp.IsSuccess() {
		apiErr := parseOpenWeatherError(resp)
		complete(apiErr)
		return nil, apiErr
	}

	complete(nil)
	return &weatherResp, nil
}

// convertToStringMap converts map[string]interface{} to map[string]string for resty
func convertToStringMap(input map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for key, value := range input {
		result[key] = fmt.Sprintf("%v", value)
	}
	return result
}

// parseOpenWeatherError creates appropriate error from API response
func parseOpenWeatherError(resp *resty.Response) error {
	statusCode := resp.StatusCode()

	// Try to parse error response if JSON
	var apiError struct {
		Cod     int    `json:"cod"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(resp.Body(), &apiError); err == nil && apiError.Message != "" {
		return &OpenWeatherAPIError{
			StatusCode: statusCode,
			Code:       apiError.Cod,
			Message:    apiError.Message,
		}
	}

	// Fallback to status-based error messages
	switch statusCode {
	case 401:
		return &OpenWeatherAPIError{
			StatusCode: statusCode,
			Code:       401,
			Message:    "Invalid API key. Please verify your OpenWeather API key.",
		}
	case 404:
		return &OpenWeatherAPIError{
			StatusCode: statusCode,
			Code:       404,
			Message:    "Location not found. Please check your coordinates.",
		}
	case 429:
		return &OpenWeatherAPIError{
			StatusCode: statusCode,
			Code:       429,
			Message:    "API rate limit exceeded. Please try again later.",
		}
	default:
		return &OpenWeatherAPIError{
			StatusCode: statusCode,
			Code:       statusCode,
			Message:    fmt.Sprintf("API request failed with status %d", statusCode),
		}
	}
}

// OpenWeatherAPIError represents an error response from the OpenWeather API
type OpenWeatherAPIError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *OpenWeatherAPIError) Error() string {
	return fmt.Sprintf("OpenWeather API error (code %d): %s", e.Code, e.Message)
}

// ForecastResponse represents the complete OpenWeather 5-day forecast API response
type ForecastResponse struct {
	Cod     string         `json:"cod"`     // Response code
	Message int            `json:"message"` // Internal parameter
	Cnt     int            `json:"cnt"`     // Number of forecast entries
	List    []ForecastItem `json:"list"`    // Array of forecast data
	City    CityInfo       `json:"city"`    // City information
}

// ForecastItem represents a single forecast entry (typically 3-hour intervals)
type ForecastItem struct {
	Dt         int64              `json:"dt"`             // Unix timestamp
	Main       MainWeatherData    `json:"main"`           // Temperature, pressure, humidity data
	Weather    []WeatherCondition `json:"weather"`        // Weather conditions array
	Clouds     CloudData          `json:"clouds"`         // Cloud coverage data
	Wind       WindData           `json:"wind"`           // Wind data
	Visibility int                `json:"visibility"`     // Visibility in meters
	Pop        float64            `json:"pop"`            // Probability of precipitation (0-1)
	Rain       *PrecipitationData `json:"rain,omitempty"` // Rain data (if present)
	Snow       *PrecipitationData `json:"snow,omitempty"` // Snow data (if present)
	Sys        SystemData         `json:"sys"`            // System data
	DtTxt      string             `json:"dt_txt"`         // Forecast time as string
}

// MainWeatherData contains temperature, pressure, and humidity information
type MainWeatherData struct {
	Temp      float64 `json:"temp"`       // Current temperature
	FeelsLike float64 `json:"feels_like"` // Human perception of temperature
	TempMin   float64 `json:"temp_min"`   // Minimum temperature in forecast period
	TempMax   float64 `json:"temp_max"`   // Maximum temperature in forecast period
	Pressure  float64 `json:"pressure"`   // Atmospheric pressure at sea level (hPa)
	SeaLevel  float64 `json:"sea_level"`  // Atmospheric pressure at sea level (hPa)
	GrndLevel float64 `json:"grnd_level"` // Atmospheric pressure at ground level (hPa)
	Humidity  int     `json:"humidity"`   // Humidity percentage
	TempKf    float64 `json:"temp_kf"`    // Temperature adjustment factor
}

// WeatherCondition represents weather condition details
type WeatherCondition struct {
	Id          int    `json:"id"`          // Weather condition ID
	Main        string `json:"main"`        // Group of weather parameters (Rain, Snow, Extreme etc.)
	Description string `json:"description"` // Weather condition description
	Icon        string `json:"icon"`        // Weather icon ID
}

// CloudData contains cloud coverage information
type CloudData struct {
	All int `json:"all"` // Cloud coverage percentage (0-100)
}

// WindData contains wind speed and direction information
type WindData struct {
	Speed float64 `json:"speed"` // Wind speed (units depend on request units)
	Deg   float64 `json:"deg"`   // Wind direction in degrees (0-360)
	Gust  float64 `json:"gust"`  // Wind gust speed (units depend on request units)
}

// PrecipitationData contains precipitation volume information
type PrecipitationData struct {
	OneHour   float64 `json:"1h,omitempty"` // Precipitation volume for the last 1 hour (mm)
	ThreeHour float64 `json:"3h,omitempty"` // Precipitation volume for the last 3 hours (mm)
}

// SystemData contains system information
type SystemData struct {
	Pod string `json:"pod"` // Part of the day (n - night, d - day)
}

// CityInfo contains city information from the API response
type CityInfo struct {
	Id         int         `json:"id"`         // City ID
	Name       string      `json:"name"`       // City name
	Coord      Coordinates `json:"coord"`      // City coordinates
	Country    string      `json:"country"`    // Country code (ISO 3166)
	Population int         `json:"population"` // City population
	Timezone   int         `json:"timezone"`   // Timezone offset in seconds from UTC
	Sunrise    int64       `json:"sunrise"`    // Sunrise time (Unix timestamp)
	Sunset     int64       `json:"sunset"`     // Sunset time (Unix timestamp)
}

// Coordinates represents latitude and longitude
type Coordinates struct {
	Lat float64 `json:"lat"` // Latitude
	Lon float64 `json:"lon"` // Longitude
}

// TodayWeatherData contains processed weather data for the current day
type TodayWeatherData struct {
	TempHigh          float64   `json:"temp_high"`          // Highest temperature today
	TempLow           float64   `json:"temp_low"`           // Lowest temperature today
	CurrentTemp       float64   `json:"current_temp"`       // Current temperature (closest to now)
	CurrentConditions string    `json:"current_conditions"` // Current weather description
	RainChance        float64   `json:"rain_chance"`        // Maximum precipitation probability
	WindConditions    string    `json:"wind_conditions"`    // Wind speed and direction description
	WeatherAlerts     []string  `json:"weather_alerts"`     // Notable weather conditions
	LastUpdated       time.Time `json:"last_updated"`       // When data was processed
	Units             string    `json:"units"`              // Unit system used
	Location          string    `json:"location"`           // Location name
	Country           string    `json:"country"`            // Country code
}

// isNotableWeatherCondition determines if a weather condition should be included in alerts
func isNotableWeatherCondition(condition string) bool {
	notableConditions := map[string]bool{
		"Thunderstorm": true,
		"Rain":         true,
		"Snow":         true,
		"Drizzle":      true,
		"Mist":         true,
		"Fog":          true,
		"Haze":         true,
		"Dust":         true,
		"Sand":         true,
		"Ash":          true,
		"Squall":       true,
		"Tornado":      true,
	}

	return notableConditions[condition]
}

// formatWindConditions creates a human-readable wind description
func formatWindConditions(wind WindData) string {
	if wind.Speed == 0 {
		return "Calm"
	}

	// Convert wind direction degrees to cardinal direction
	direction := degreesToCardinal(wind.Deg)

	// Describe wind speed (these thresholds work for both m/s and mph roughly)
	var speedDesc string
	switch {
	case wind.Speed < 2:
		speedDesc = "Light"
	case wind.Speed < 6:
		speedDesc = "Gentle"
	case wind.Speed < 12:
		speedDesc = "Moderate"
	case wind.Speed < 20:
		speedDesc = "Fresh"
	case wind.Speed < 30:
		speedDesc = "Strong"
	default:
		speedDesc = "Very Strong"
	}

	result := fmt.Sprintf("%s %s winds at %.1f", speedDesc, direction, wind.Speed)

	// Add gust information if significant
	if wind.Gust > wind.Speed*1.5 {
		result += fmt.Sprintf(" (gusts to %.1f)", wind.Gust)
	}

	return result
}

// degreesToCardinal converts wind direction degrees to cardinal direction
func degreesToCardinal(degrees float64) string {
	// Normalize degrees to 0-360 range
	degrees = float64(int(degrees+360) % 360)

	directions := []string{
		"N", "NNE", "NE", "ENE",
		"E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW",
		"W", "WNW", "NW", "NNW",
	}

	// Each direction covers 22.5 degrees
	index := int((degrees+11.25)/22.5) % 16
	return directions[index]
}

// ConvertTemperature converts temperature between different unit systems
func ConvertTemperature(temp float64, fromUnit, toUnit string) float64 {
	// AIDEV-NOTE: Temperature conversion handles Celsius, Fahrenheit, and Kelvin
	if fromUnit == toUnit {
		return temp
	}

	// First convert to Celsius as intermediate
	var tempC float64
	switch strings.ToLower(fromUnit) {
	case "fahrenheit", "imperial":
		tempC = (temp - 32) * 5 / 9
	case "kelvin":
		tempC = temp - 273.15
	default:
		tempC = temp // Assume Celsius
	}

	// Then convert from Celsius to target unit
	switch strings.ToLower(toUnit) {
	case "fahrenheit", "imperial":
		return tempC*9/5 + 32
	case "kelvin":
		return tempC + 273.15
	default:
		return tempC // Return Celsius
	}
}

// ConvertWindSpeed converts wind speed between different unit systems
func ConvertWindSpeed(speed float64, fromUnit, toUnit string) float64 {
	// AIDEV-NOTE: Wind speed conversion between m/s, mph, and km/h
	if fromUnit == toUnit {
		return speed
	}

	// First convert to m/s as intermediate
	var speedMS float64
	switch strings.ToLower(fromUnit) {
	case "mph", "imperial":
		speedMS = speed * 0.44704 // mph to m/s
	case "kmh", "km/h":
		speedMS = speed * 0.277778 // km/h to m/s
	default:
		speedMS = speed // Assume m/s
	}

	// Then convert from m/s to target unit
	switch strings.ToLower(toUnit) {
	case "mph", "imperial":
		return speedMS * 2.237 // m/s to mph
	case "kmh", "km/h":
		return speedMS * 3.6 // m/s to km/h
	default:
		return speedMS // Return m/s
	}
}

// ConvertPressure converts atmospheric pressure between different units
func ConvertPressure(pressure float64, fromUnit, toUnit string) float64 {
	// AIDEV-NOTE: Pressure conversion between hPa, inHg, and mmHg
	if fromUnit == toUnit {
		return pressure
	}

	// First convert to hPa as intermediate
	var pressureHPa float64
	switch strings.ToLower(fromUnit) {
	case "inhg", "inches":
		pressureHPa = pressure * 33.8639 // inHg to hPa
	case "mmhg", "torr":
		pressureHPa = pressure * 1.33322 // mmHg to hPa
	default:
		pressureHPa = pressure // Assume hPa
	}

	// Then convert from hPa to target unit
	switch strings.ToLower(toUnit) {
	case "inhg", "inches":
		return pressureHPa * 0.02953 // hPa to inHg
	case "mmhg", "torr":
		return pressureHPa * 0.75006 // hPa to mmHg
	default:
		return pressureHPa // Return hPa
	}
}

// ConvertWeatherData converts all weather measurements to the specified unit system
func (w *WeatherClient) ConvertWeatherData(data *TodayWeatherData, targetUnits string) *TodayWeatherData {
	if data == nil || data.Units == targetUnits {
		return data
	}

	// AIDEV-NOTE: Deep copy to avoid modifying original data
	converted := &TodayWeatherData{
		TempHigh:          ConvertTemperature(data.TempHigh, data.Units, targetUnits),
		TempLow:           ConvertTemperature(data.TempLow, data.Units, targetUnits),
		CurrentTemp:       ConvertTemperature(data.CurrentTemp, data.Units, targetUnits),
		CurrentConditions: data.CurrentConditions,
		RainChance:        data.RainChance,                           // Percentage stays the same
		WeatherAlerts:     append([]string{}, data.WeatherAlerts...), // Copy slice
		LastUpdated:       data.LastUpdated,
		Units:             targetUnits,
		Location:          data.Location,
	}

	// Convert wind conditions description if it contains numerical values
	converted.WindConditions = convertWindInDescription(data.WindConditions, data.Units, targetUnits)

	return converted
}

// convertWindInDescription updates wind speed units in the wind conditions description
func convertWindInDescription(windDesc, fromUnits, toUnits string) string {
	if fromUnits == toUnits {
		return windDesc
	}

	// Simple approach: find numerical patterns and convert them
	// This handles the format from formatWindConditions function
	// Example: "Moderate NW winds at 15.0 (gusts to 22.0)"

	// Use regex to find numerical values that represent wind speeds
	re := regexp.MustCompile(`(\d+\.?\d*)\s*(?:\(gusts to (\d+\.?\d*)\))?`)

	return re.ReplaceAllStringFunc(windDesc, func(match string) string {
		// Extract wind speed and gust values
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		// Convert main wind speed
		if speed, err := strconv.ParseFloat(submatches[1], 64); err == nil {
			convertedSpeed := ConvertWindSpeed(speed, fromUnits, toUnits)
			result := fmt.Sprintf("%.1f", convertedSpeed)

			// Convert gust speed if present
			if len(submatches) > 2 && submatches[2] != "" {
				if gust, err := strconv.ParseFloat(submatches[2], 64); err == nil {
					convertedGust := ConvertWindSpeed(gust, fromUnits, toUnits)
					result += fmt.Sprintf(" (gusts to %.1f)", convertedGust)
				}
			}

			return result
		}

		return match
	})
}

// GetUnitSuffix returns the appropriate unit suffix for display
func GetUnitSuffix(measurement, units string) string {
	switch strings.ToLower(measurement) {
	case "temperature", "temp":
		switch strings.ToLower(units) {
		case "imperial", "fahrenheit":
			return "°F"
		case "kelvin":
			return "K"
		default:
			return "°C"
		}
	case "wind", "speed":
		switch strings.ToLower(units) {
		case "imperial":
			return "mph"
		case "kmh", "km/h":
			return "km/h"
		default:
			return "m/s"
		}
	case "pressure":
		switch strings.ToLower(units) {
		case "imperial", "inches":
			return "inHg"
		case "mmhg", "torr":
			return "mmHg"
		default:
			return "hPa"
		}
	case "precipitation", "rain":
		return "mm" // Precipitation is typically in mm regardless of unit system
	default:
		return ""
	}
}

// RateLimiter handles API rate limiting to respect OpenWeather API limits
type RateLimiter struct {
	requests    []time.Time
	maxRequests int
	window      time.Duration
}

// NewRateLimiter creates a new rate limiter with specified limits
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests:    make([]time.Time, 0),
		maxRequests: maxRequests,
		window:      window,
	}
}

// Wait blocks until a request can be made according to rate limits
func (rl *RateLimiter) Wait(ctx context.Context) error {
	now := time.Now()

	// Remove requests outside the time window
	cutoff := now.Add(-rl.window)
	i := 0
	for i < len(rl.requests) && rl.requests[i].Before(cutoff) {
		i++
	}
	rl.requests = rl.requests[i:]

	// Check if we can make a request
	if len(rl.requests) < rl.maxRequests {
		rl.requests = append(rl.requests, now)
		return nil
	}

	// Wait until we can make a request
	sleepTime := rl.requests[0].Add(rl.window).Sub(now)
	if sleepTime > 0 {
		logger.Debug("Rate limit reached, waiting %.2f seconds", sleepTime.Seconds())

		select {
		case <-time.After(sleepTime):
			rl.requests = append(rl.requests[1:], now)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	rl.requests = append(rl.requests, now)
	return nil
}

// WeatherClientWithRateLimit extends WeatherClient with rate limiting
type WeatherClientWithRateLimit struct {
	*WeatherClient
	rateLimiter *RateLimiter
}

// NewWeatherClientWithRateLimit creates a weather client with built-in rate limiting
// OpenWeather free tier allows 1000 requests per day, 60 per minute
func NewWeatherClientWithRateLimit(apiKey string) *WeatherClientWithRateLimit {
	client := NewWeatherClient(apiKey)
	// Set conservative rate limit: 50 requests per minute to stay well under limit
	rateLimiter := NewRateLimiter(50, time.Minute)

	return &WeatherClientWithRateLimit{
		WeatherClient: client,
		rateLimiter:   rateLimiter,
	}
}

// validateForecastParams validates input parameters for API requests
func validateForecastParams(params ForecastParams) error {
	var errors []string

	// Validate latitude range
	if params.Latitude < -90 || params.Latitude > 90 {
		errors = append(errors, fmt.Sprintf("latitude must be between -90 and 90, got %.6f", params.Latitude))
	}

	// Validate longitude range
	if params.Longitude < -180 || params.Longitude > 180 {
		errors = append(errors, fmt.Sprintf("longitude must be between -180 and 180, got %.6f", params.Longitude))
	}

	// Validate units
	validUnits := map[string]bool{
		"metric":   true,
		"imperial": true,
		"kelvin":   true,
	}
	if params.Units != "" && !validUnits[strings.ToLower(params.Units)] {
		errors = append(errors, fmt.Sprintf("units must be one of: metric, imperial, kelvin, got '%s'", params.Units))
	}

	// Validate count (OpenWeather API limit)
	if params.Count > 0 && params.Count > 40 {
		errors = append(errors, fmt.Sprintf("count cannot exceed 40, got %d", params.Count))
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for OpenWeather API specific errors
	if apiErr, ok := err.(*OpenWeatherAPIError); ok {
		switch apiErr.StatusCode {
		case 429: // Rate limit exceeded
			return true
		case 500, 502, 503, 504: // Server errors
			return true
		case 401, 403: // Authentication errors - not retryable
			return false
		case 404: // Not found - not retryable
			return false
		default:
			return false
		}
	}

	// Check for network-related errors (context cancelled, timeouts, etc.)
	errorString := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"network unreachable",
		"temporary failure",
		"i/o timeout",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errorString, pattern) {
			return true
		}
	}

	return false
}

// GetCurrentWeatherWithRateLimit fetches current weather data with rate limiting
func (w *WeatherClientWithRateLimit) GetCurrentWeatherWithRateLimit(ctx context.Context, params ForecastParams) (*CurrentWeatherResponse, error) {
	// Apply rate limiting
	if err := w.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter cancelled: %w", err)
	}

	// Validate input parameters before making request
	if err := validateForecastParams(params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	return w.WeatherClient.GetCurrentWeather(ctx, params)
}

// GetOneCallWeatherWithRateLimit fetches One Call API data with rate limiting
func (w *WeatherClientWithRateLimit) GetOneCallWeatherWithRateLimit(ctx context.Context, params ForecastParams) (*OneCallResponse, error) {
	// Apply rate limiting
	if err := w.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter cancelled: %w", err)
	}

	// Validate input parameters before making request
	if err := validateForecastParams(params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	return w.WeatherClient.GetOneCallWeather(ctx, params)
}

// GetTodayWeatherWithOneCall fetches weather using One Call API 3.0
func (w *WeatherClientWithRateLimit) GetTodayWeatherWithOneCall(ctx context.Context, params ForecastParams, targetUnits string) (*TodayWeatherData, error) {
	complete := logger.LogOperationStart("get_today_weather_onecall", map[string]any{
		"latitude":     params.Latitude,
		"longitude":    params.Longitude,
		"units":        params.Units,
		"target_units": targetUnits,
	})

	// Get One Call data with rate limiting and retries
	oneCall, err := w.GetOneCallWeatherWithRateLimit(ctx, params)
	if err != nil {
		complete(fmt.Errorf("failed to fetch One Call data: %w", err))
		return nil, fmt.Errorf("unable to fetch weather data: %w", err)
	}

	// Extract today's weather data
	todayData, err := w.ExtractTodayWeatherFromOneCallWithContext(ctx, oneCall)
	if err != nil {
		complete(fmt.Errorf("failed to extract today's data: %w", err))
		return nil, fmt.Errorf("unable to process weather data: %w", err)
	}

	// Set proper units
	todayData.Units = params.Units

	// Convert units if needed
	if targetUnits != "" && targetUnits != todayData.Units {
		todayData = w.ConvertWeatherData(todayData, targetUnits)
	}

	complete(nil)
	return todayData, nil
}

// GetTodayWeatherWithOneCallCache fetches weather using One Call API with caching
func (w *WeatherClientWithRateLimit) GetTodayWeatherWithOneCallCache(ctx context.Context, params ForecastParams, targetUnits string, cacheManager *CacheManager) (*TodayWeatherData, *OneCallResponse, error) {
	complete := logger.LogOperationStart("get_today_weather_onecall_cache", map[string]any{
		"latitude":      params.Latitude,
		"longitude":     params.Longitude,
		"units":         params.Units,
		"target_units":  targetUnits,
		"cache_enabled": cacheManager != nil,
	})

	var oneCall *OneCallResponse
	var todayData *TodayWeatherData

	// Check if cache is valid for today
	if cacheManager != nil && cacheManager.IsValidForToday() {
		logger.Debug("Using cached weather data for daily forecast values")

		// Try to read cache
		cache, err := cacheManager.Read()
		if err == nil {
			// Verify cache is for the same location
			if cache.Latitude == params.Latitude && cache.Longitude == params.Longitude && cache.Units == params.Units {
				// Fetch fresh One Call data for current conditions
				oneCall, err = w.GetOneCallWeatherWithRateLimit(ctx, params)
				if err != nil {
					logger.Warn("Failed to fetch One Call data, falling back to full fetch: %v", err)
				} else {
					// Use cached daily min/max with fresh current conditions
					todayData = &TodayWeatherData{
						TempHigh:          cache.DailyForecast.TempHigh, // Cached daily high
						TempLow:           cache.DailyForecast.TempLow,  // Cached daily low
						CurrentTemp:       oneCall.Current.Temp,         // Fresh current temp
						CurrentConditions: "",
						RainChance:        0.0,
						WindConditions:    "",
						WeatherAlerts:     []string{},
						LastUpdated:       time.Now(),
						Units:             params.Units,
						Location:          cache.Location,
						Country:           cache.DailyForecast.Country, // Cached country
					}

					// Extract fresh current conditions
					if len(oneCall.Current.Weather) > 0 {
						todayData.CurrentConditions = oneCall.Current.Weather[0].Description
					}

					// Format fresh wind conditions
					todayData.WindConditions = formatWindConditions(WindData{
						Speed: oneCall.Current.WindSpeed,
						Deg:   float64(oneCall.Current.WindDeg),
						Gust:  oneCall.Current.WindGust,
					})

					// Get fresh rain chance from today's daily forecast
					if len(oneCall.Daily) > 0 {
						todayData.RainChance = oneCall.Daily[0].Pop
					}

					// Check for fresh weather alerts
					if len(oneCall.Alerts) > 0 {
						for _, alert := range oneCall.Alerts {
							todayData.WeatherAlerts = append(todayData.WeatherAlerts, alert.Event)
						}
					}

					// Convert units if needed
					if targetUnits != "" && targetUnits != todayData.Units {
						todayData = w.ConvertWeatherData(todayData, targetUnits)
					}

					logger.Debug("Successfully used cached daily values with fresh current conditions")
					complete(nil)
					return todayData, oneCall, nil
				}
			} else {
				logger.Debug("Cache location mismatch, fetching fresh data")
			}
		} else {
			logger.Debug("Failed to read cache: %v", err)
		}
	}

	// Fallback: fetch complete fresh data
	logger.Debug("Fetching fresh One Call weather data")
	oneCall, err := w.GetOneCallWeatherWithRateLimit(ctx, params)
	if err != nil {
		complete(fmt.Errorf("failed to fetch One Call data: %w", err))
		return nil, nil, fmt.Errorf("unable to fetch weather data: %w", err)
	}

	// Extract today's weather data
	todayData, err = w.ExtractTodayWeatherFromOneCallWithContext(ctx, oneCall)
	if err != nil {
		complete(fmt.Errorf("failed to extract today's data: %w", err))
		return nil, nil, fmt.Errorf("unable to process weather data: %w", err)
	}

	// Set proper units and location
	todayData.Units = params.Units

	// Convert units if needed
	if targetUnits != "" && targetUnits != todayData.Units {
		todayData = w.ConvertWeatherData(todayData, targetUnits)
	}

	// Save to cache if manager is available
	if cacheManager != nil && oneCall != nil {
		logger.Debug("Saving fresh One Call data to cache")
		if err := cacheManager.WriteOneCall(oneCall, todayData); err != nil {
			logger.Warn("Failed to save weather cache: %v", err)
			// Continue despite cache write failure
		}
	}

	complete(nil)
	return todayData, oneCall, nil
}

// GeocodingResponse represents the OpenWeather Geocoding API response
type GeocodingResponse struct {
	Name       string            `json:"name"`
	LocalNames map[string]string `json:"local_names,omitempty"`
	Country    string            `json:"country"`
	State      string            `json:"state,omitempty"`
}

// LocationInfo holds geocoded location details
type LocationInfo struct {
	City    string
	State   string
	Country string
	Display string // Formatted display name
}

// GetLocationName performs reverse geocoding to get city name from coordinates
func (w *WeatherClient) GetLocationName(ctx context.Context, lat, lon float64) string {
	info := w.GetLocationInfo(ctx, lat, lon)
	return info.Display
}

// GetLocationInfo performs reverse geocoding to get full location details from coordinates
func (w *WeatherClient) GetLocationInfo(ctx context.Context, lat, lon float64) LocationInfo {
	// Build query parameters for reverse geocoding
	queryParams := map[string]string{
		"lat":   fmt.Sprintf("%f", lat),
		"lon":   fmt.Sprintf("%f", lon),
		"limit": "1",
		"appid": w.apiKey,
	}

	// Create request
	req := w.client.R().
		SetContext(ctx).
		SetQueryParams(queryParams).
		SetHeader("User-Agent", userAgent)

	// Make the API call
	resp, err := req.Get(geocodingBaseURL + reverseGeoEndpoint)
	if err != nil {
		logger.Debug("Reverse geocoding failed: %v", err)
		return LocationInfo{}
	}

	if resp.StatusCode() != 200 {
		logger.Debug("Reverse geocoding returned status %d", resp.StatusCode())
		return LocationInfo{}
	}

	// Parse response
	var geoData []GeocodingResponse
	if err := json.Unmarshal(resp.Body(), &geoData); err != nil {
		logger.Debug("Failed to parse geocoding response: %v", err)
		return LocationInfo{}
	}

	if len(geoData) == 0 {
		logger.Debug("No geocoding results found")
		return LocationInfo{}
	}

	// Build location info
	info := LocationInfo{
		City:    geoData[0].Name,
		State:   geoData[0].State,
		Country: geoData[0].Country,
	}

	// Format display name
	if info.State != "" {
		info.Display = fmt.Sprintf("%s, %s", info.City, info.State)
	} else if info.Country != "" {
		info.Display = fmt.Sprintf("%s, %s", info.City, info.Country)
	} else {
		info.Display = info.City
	}

	logger.Debug("Reverse geocoding successful: %s (Country: %s)", info.Display, info.Country)
	return info
}
