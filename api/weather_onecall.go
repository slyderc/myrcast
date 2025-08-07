package api

import (
	"context"
	"fmt"
	"time"

	"myrcast/internal/logger"
)

const (
	// One Call API 3.0 endpoint
	oneCallBaseURL  = "https://api.openweathermap.org/data/3.0"
	oneCallEndpoint = "/onecall"
)

// OneCallResponse represents the complete One Call API 3.0 response
type OneCallResponse struct {
	Lat            float64        `json:"lat"`
	Lon            float64        `json:"lon"`
	Timezone       string         `json:"timezone"`
	TimezoneOffset int            `json:"timezone_offset"`
	Current        CurrentData    `json:"current"`
	Minutely       []MinutelyData `json:"minutely,omitempty"`
	Hourly         []HourlyData   `json:"hourly,omitempty"`
	Daily          []DailyData    `json:"daily"`
	Alerts         []WeatherAlert `json:"alerts,omitempty"`
}

// CurrentData represents current weather conditions
type CurrentData struct {
	Dt         int64              `json:"dt"`
	Sunrise    int64              `json:"sunrise"`
	Sunset     int64              `json:"sunset"`
	Temp       float64            `json:"temp"`
	FeelsLike  float64            `json:"feels_like"`
	Pressure   int                `json:"pressure"`
	Humidity   int                `json:"humidity"`
	DewPoint   float64            `json:"dew_point"`
	Uvi        float64            `json:"uvi"`
	Clouds     int                `json:"clouds"`
	Visibility int                `json:"visibility"`
	WindSpeed  float64            `json:"wind_speed"`
	WindDeg    int                `json:"wind_deg"`
	WindGust   float64            `json:"wind_gust,omitempty"`
	Weather    []WeatherCondition `json:"weather"`
	Rain       *RainData          `json:"rain,omitempty"`
	Snow       *SnowData          `json:"snow,omitempty"`
}

// MinutelyData represents minute-by-minute precipitation forecast
type MinutelyData struct {
	Dt            int64   `json:"dt"`
	Precipitation float64 `json:"precipitation"`
}

// HourlyData represents hourly forecast data
type HourlyData struct {
	Dt         int64              `json:"dt"`
	Temp       float64            `json:"temp"`
	FeelsLike  float64            `json:"feels_like"`
	Pressure   int                `json:"pressure"`
	Humidity   int                `json:"humidity"`
	DewPoint   float64            `json:"dew_point"`
	Uvi        float64            `json:"uvi"`
	Clouds     int                `json:"clouds"`
	Visibility int                `json:"visibility"`
	WindSpeed  float64            `json:"wind_speed"`
	WindDeg    int                `json:"wind_deg"`
	WindGust   float64            `json:"wind_gust,omitempty"`
	Weather    []WeatherCondition `json:"weather"`
	Pop        float64            `json:"pop"`
	Rain       *RainData          `json:"rain,omitempty"`
	Snow       *SnowData          `json:"snow,omitempty"`
}

// DailyData represents daily forecast data with proper min/max temperatures
type DailyData struct {
	Dt        int64              `json:"dt"`
	Sunrise   int64              `json:"sunrise"`
	Sunset    int64              `json:"sunset"`
	Moonrise  int64              `json:"moonrise"`
	Moonset   int64              `json:"moonset"`
	MoonPhase float64            `json:"moon_phase"`
	Summary   string             `json:"summary,omitempty"`
	Temp      DailyTemperature   `json:"temp"`
	FeelsLike DailyFeelsLike     `json:"feels_like"`
	Pressure  int                `json:"pressure"`
	Humidity  int                `json:"humidity"`
	DewPoint  float64            `json:"dew_point"`
	WindSpeed float64            `json:"wind_speed"`
	WindDeg   int                `json:"wind_deg"`
	WindGust  float64            `json:"wind_gust,omitempty"`
	Weather   []WeatherCondition `json:"weather"`
	Clouds    int                `json:"clouds"`
	Pop       float64            `json:"pop"`
	Rain      float64            `json:"rain,omitempty"`
	Snow      float64            `json:"snow,omitempty"`
	Uvi       float64            `json:"uvi"`
}

// DailyTemperature contains temperature data throughout the day
type DailyTemperature struct {
	Day   float64 `json:"day"`   // Day temperature
	Min   float64 `json:"min"`   // Daily minimum temperature
	Max   float64 `json:"max"`   // Daily maximum temperature
	Night float64 `json:"night"` // Night temperature
	Eve   float64 `json:"eve"`   // Evening temperature
	Morn  float64 `json:"morn"`  // Morning temperature
}

// DailyFeelsLike contains "feels like" temperature data
type DailyFeelsLike struct {
	Day   float64 `json:"day"`
	Night float64 `json:"night"`
	Eve   float64 `json:"eve"`
	Morn  float64 `json:"morn"`
}

// RainData represents rain volume
type RainData struct {
	OneHour float64 `json:"1h,omitempty"`
}

// SnowData represents snow volume
type SnowData struct {
	OneHour float64 `json:"1h,omitempty"`
}

// WeatherAlert represents weather alerts
type WeatherAlert struct {
	SenderName  string   `json:"sender_name"`
	Event       string   `json:"event"`
	Start       int64    `json:"start"`
	End         int64    `json:"end"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// GetOneCallWeather fetches weather data from One Call API 3.0
func (w *WeatherClient) GetOneCallWeather(ctx context.Context, params ForecastParams) (*OneCallResponse, error) {
	complete := logger.LogOperationStart("weather_api_onecall", map[string]any{
		"endpoint":  "onecall",
		"latitude":  params.Latitude,
		"longitude": params.Longitude,
		"units":     params.Units,
	})

	// Build query parameters for the API request
	queryParams := map[string]string{
		"lat":   fmt.Sprintf("%f", params.Latitude),
		"lon":   fmt.Sprintf("%f", params.Longitude),
		"appid": w.apiKey,
		"units": params.Units,
		// Exclude minutely and hourly data to reduce response size
		"exclude": "minutely,hourly",
	}

	var response OneCallResponse

	// Execute the HTTP request with context for cancellation
	// Note: Using a new client instance for One Call API 3.0
	client := w.client.Clone()
	client.SetBaseURL(oneCallBaseURL)

	resp, err := client.R().
		SetContext(ctx).
		SetQueryParams(queryParams).
		SetResult(&response).
		Get(oneCallEndpoint)

	if err != nil {
		complete(fmt.Errorf("HTTP request failed: %w", err))
		return nil, fmt.Errorf("failed to fetch One Call weather: %w", err)
	}

	// Check for HTTP error status codes
	if !resp.IsSuccess() {
		apiErr := parseOpenWeatherError(resp)
		complete(apiErr)
		return nil, apiErr
	}

	complete(nil)
	logger.Debug("One Call API response received: location=(%f,%f), current_temp=%.1f, daily_count=%d",
		response.Lat, response.Lon, response.Current.Temp, len(response.Daily))

	return &response, nil
}

// ExtractTodayWeatherFromOneCall processes One Call API data to extract today's weather
func (w *WeatherClient) ExtractTodayWeatherFromOneCall(oneCall *OneCallResponse) (*TodayWeatherData, error) {
	return w.ExtractTodayWeatherFromOneCallWithContext(context.Background(), oneCall)
}

// ExtractTodayWeatherFromOneCallWithContext processes One Call API data to extract today's weather with context
func (w *WeatherClient) ExtractTodayWeatherFromOneCallWithContext(ctx context.Context, oneCall *OneCallResponse) (*TodayWeatherData, error) {
	if oneCall == nil || len(oneCall.Daily) == 0 {
		return nil, fmt.Errorf("empty One Call data")
	}

	complete := logger.LogOperationStart("weather_data_extraction_onecall", map[string]any{
		"timezone":    oneCall.Timezone,
		"daily_count": len(oneCall.Daily),
	})

	// Get today's daily forecast (first element in daily array)
	todayDaily := oneCall.Daily[0]

	// Get current conditions
	currentConditions := "Clear"
	if len(oneCall.Current.Weather) > 0 {
		currentConditions = oneCall.Current.Weather[0].Description
	}

	// Format wind conditions
	windConditions := formatWindConditions(WindData{
		Speed: oneCall.Current.WindSpeed,
		Deg:   float64(oneCall.Current.WindDeg),
		Gust:  oneCall.Current.WindGust,
	})

	// Check for weather alerts
	var weatherAlerts []string
	if len(oneCall.Alerts) > 0 {
		for _, alert := range oneCall.Alerts {
			weatherAlerts = append(weatherAlerts, alert.Event)
		}
	}

	// Determine unit system from configuration (already specified in request)
	units := "metric" // Default, but should match params.Units

	// Try to get the actual location details via reverse geocoding
	locationInfo := w.GetLocationInfo(ctx, oneCall.Lat, oneCall.Lon)
	locationName := locationInfo.Display
	if locationName == "" {
		// Fallback to timezone if geocoding fails
		locationName = oneCall.Timezone
	}

	result := &TodayWeatherData{
		TempHigh:          todayDaily.Temp.Max,  // Proper daily maximum
		TempLow:           todayDaily.Temp.Min,  // Proper daily minimum
		CurrentTemp:       oneCall.Current.Temp, // Real-time current temperature
		CurrentConditions: currentConditions,
		RainChance:        todayDaily.Pop, // Probability of precipitation (0-1)
		WindConditions:    windConditions,
		WeatherAlerts:     weatherAlerts,
		LastUpdated:       time.Now(),
		Units:             units,
		Location:          locationName,
		Country:           locationInfo.Country,
	}

	complete(nil)
	logger.Debug("Today's weather extracted from One Call: high=%.1f, low=%.1f, current=%.1f, conditions=%s",
		result.TempHigh, result.TempLow, result.CurrentTemp, result.CurrentConditions)

	return result, nil
}
