package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"myrcast/internal/logger"
)

const (
	// Default values for Claude API
	defaultModel         = "claude-3-5-sonnet-20241022"
	defaultMaxTokens     = 1000
	defaultTemperature   = 0.7
	defaultClaudeTimeout = 30 * time.Second

	// Retry configuration
	defaultMaxRetries   = 3
	defaultBaseDelay    = 1 * time.Second
	defaultMaxDelay     = 30 * time.Second
	defaultJitterFactor = 0.1

	// Rate limiting
	defaultRateLimit = 50 // requests per minute (conservative for Anthropic API)
)

// ClaudeClient handles Anthropic Claude API interactions
type ClaudeClient struct {
	client      anthropic.Client
	config      ClaudeConfig
	rateLimiter *ClaudeRateLimiter
}

// ClaudeConfig contains configuration for Claude API client
type ClaudeConfig struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RateLimit   int // requests per minute
}

// ClaudeRateLimiter handles rate limiting for Claude API requests
type ClaudeRateLimiter struct {
	requests    []time.Time
	maxRequests int
	window      time.Duration
}

// NewClaudeRateLimiter creates a new rate limiter for Claude API
func NewClaudeRateLimiter(requestsPerMinute int) *ClaudeRateLimiter {
	return &ClaudeRateLimiter{
		requests:    make([]time.Time, 0),
		maxRequests: requestsPerMinute,
		window:      time.Minute,
	}
}

// Wait blocks until a request can be made according to rate limits
func (rl *ClaudeRateLimiter) Wait(ctx context.Context) error {
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
		logger.LogWithFields(logger.DebugLevel, "Claude API rate limit reached, waiting", map[string]any{
			"wait_seconds": sleepTime.Seconds(),
		})

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

// ClaudeAPIError represents errors from the Claude API
type ClaudeAPIError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StatusCode int
	Retryable  bool
}

func (e *ClaudeAPIError) Error() string {
	return fmt.Sprintf("Claude API error (status %d, type %s): %s", e.StatusCode, e.Type, e.Message)
}

// IsRetryable returns true if this error indicates a retryable condition
func (e *ClaudeAPIError) IsRetryable() bool {
	return e.Retryable
}

// NewClaudeClient creates a new Claude API client with the provided configuration
func NewClaudeClient(config ClaudeConfig) (*ClaudeClient, error) {
	// AIDEV-NOTE: Validate API key before creating client to fail fast
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("Claude API key is required")
	}

	// Validate configuration parameters
	if config.Temperature < 0 || config.Temperature > 1.0 {
		return nil, fmt.Errorf("temperature must be between 0 and 1.0, got %v", config.Temperature)
	}
	if config.MaxTokens < 0 {
		return nil, fmt.Errorf("max_tokens must be non-negative, got %v", config.MaxTokens)
	}

	// Apply defaults to configuration
	if config.Model == "" {
		config.Model = defaultModel
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaultMaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaultTemperature
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultClaudeTimeout
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = defaultMaxRetries
	}
	if config.BaseDelay <= 0 {
		config.BaseDelay = defaultBaseDelay
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = defaultMaxDelay
	}
	if config.RateLimit <= 0 {
		config.RateLimit = defaultRateLimit
	}

	// Create Anthropic client with API key
	client := anthropic.NewClient(
		option.WithAPIKey(config.APIKey),
	)

	// Create rate limiter
	rateLimiter := NewClaudeRateLimiter(config.RateLimit)

	return &ClaudeClient{
		client:      client,
		config:      config,
		rateLimiter: rateLimiter,
	}, nil
}

// WeatherReportRequest contains the request data for generating a weather report
type WeatherReportRequest struct {
	TodayData      *TodayWeatherData // Pre-extracted today's weather data from One Call API
	PromptTemplate string            // Template with variable placeholders
	Location       string            // Location name for the report
	OutputPath     string            // Directory path for logging
}

// WeatherReportResponse contains the generated weather report
type WeatherReportResponse struct {
	Script      string    // Generated weather report script
	TokensUsed  int       // Number of tokens used
	GeneratedAt time.Time // Timestamp of generation
}

// GenerateWeatherReport creates a weather report script using Claude AI with retry logic and rate limiting
func (c *ClaudeClient) GenerateWeatherReport(ctx context.Context, request WeatherReportRequest) (*WeatherReportResponse, error) {
	// AIDEV-NOTE: Enhanced with retry logic, rate limiting, and improved error handling
	complete := logger.LogOperationStart("claude_api_request_with_retry", map[string]any{
		"model":       c.config.Model,
		"max_tokens":  c.config.MaxTokens,
		"temperature": c.config.Temperature,
		"max_retries": c.config.MaxRetries,
	})

	// Use the pre-extracted today's weather data
	if request.TodayData == nil {
		complete(fmt.Errorf("today's weather data is nil"))
		return nil, fmt.Errorf("today's weather data is required but is nil")
	}

	// Format weather data for Claude context using pre-extracted data
	weatherContext, err := c.formatWeatherContextFromExtracted(request.TodayData)
	if err != nil {
		complete(fmt.Errorf("failed to format weather context: %w", err))
		return nil, fmt.Errorf("failed to format weather context: %w", err)
	}

	// Build the message request using the prompt template directly
	messageReq := anthropic.MessageNewParams{
		Model:       anthropic.Model(c.config.Model),
		MaxTokens:   int64(c.config.MaxTokens),
		Temperature: anthropic.Float(c.config.Temperature),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(request.PromptTemplate),
			),
		},
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: weatherContext,
			},
		},
	}

	// Log the full prompt information to results.log
	if err := c.logPromptToFile(request, weatherContext, messageReq); err != nil {
		logger.LogWithFields(logger.WarnLevel, "Failed to log prompt to file", map[string]any{
			"error": err.Error(),
		})
	}

	// Execute request with retry logic
	resp, err := c.executeWithRetry(ctx, messageReq)
	if err != nil {
		complete(fmt.Errorf("Claude API request failed after retries: %w", err))
		return nil, err
	}

	// Extract the generated text from response
	if len(resp.Content) == 0 {
		complete(fmt.Errorf("empty response from Claude API"))
		return nil, fmt.Errorf("empty response from Claude API")
	}

	// Extract text from the first content block
	var script string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			script = block.Text
			break
		}
	}

	if script == "" {
		complete(fmt.Errorf("no text content in Claude API response"))
		return nil, fmt.Errorf("no text content in Claude API response")
	}

	// Validate the generated script
	if err := c.validateGeneratedScript(script); err != nil {
		complete(fmt.Errorf("script validation failed: %w", err))
		return nil, fmt.Errorf("generated script validation failed: %w", err)
	}

	complete(nil)

	// Log the generated script to results.log
	if err := c.appendScriptToLog(request, script); err != nil {
		logger.LogWithFields(logger.WarnLevel, "Failed to append script to log file", map[string]any{
			"error": err.Error(),
		})
	}

	return &WeatherReportResponse{
		Script:      script,
		TokensUsed:  int(resp.Usage.OutputTokens),
		GeneratedAt: time.Now(),
	}, nil
}

// executeWithRetry executes a Claude API request with retry logic and rate limiting
func (c *ClaudeClient) executeWithRetry(ctx context.Context, messageReq anthropic.MessageNewParams) (*anthropic.Message, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Apply rate limiting before each request
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter cancelled: %w", err)
		}

		// Create context with timeout for this attempt
		reqCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)

		// Log attempt
		if attempt > 0 {
			logger.LogWithFields(logger.DebugLevel, "Retrying Claude API request", map[string]any{
				"attempt":     attempt + 1,
				"max_retries": c.config.MaxRetries + 1,
			})
		}

		// Execute the API request
		resp, err := c.client.Messages.New(reqCtx, messageReq)
		cancel() // Clean up timeout context

		if err != nil {
			lastErr = err
			claudeErr := c.parseClaudeError(err)

			// Check if this is the last attempt or if error is not retryable
			if attempt == c.config.MaxRetries || !claudeErr.IsRetryable() {
				if !claudeErr.IsRetryable() {
					logger.LogWithFields(logger.ErrorLevel, "Non-retryable Claude API error", map[string]any{
						"error":   err.Error(),
						"attempt": attempt + 1,
					})
					return nil, claudeErr
				}
				break // Exit retry loop on last attempt
			}

			// Calculate delay for next retry with exponential backoff and jitter
			delay := c.calculateRetryDelay(attempt)

			logger.LogWithFields(logger.WarnLevel, "Claude API request failed, retrying", map[string]any{
				"error":        err.Error(),
				"attempt":      attempt + 1,
				"next_attempt": attempt + 2,
				"delay_ms":     delay.Milliseconds(),
			})

			// Wait before next retry
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Success!
		if attempt > 0 {
			logger.LogWithFields(logger.DebugLevel, "Claude API request succeeded after retries", map[string]any{
				"successful_attempt": attempt + 1,
				"total_attempts":     attempt + 1,
			})
		}
		return resp, nil
	}

	// All retries exhausted
	claudeErr := c.parseClaudeError(lastErr)
	logger.LogWithFields(logger.ErrorLevel, "Claude API request failed after all retries", map[string]any{
		"total_attempts": c.config.MaxRetries + 1,
		"final_error":    lastErr.Error(),
	})

	return nil, fmt.Errorf("Claude API request failed after %d attempts: %w", c.config.MaxRetries+1, claudeErr)
}

// calculateRetryDelay calculates the delay for the next retry attempt using exponential backoff with jitter
func (c *ClaudeClient) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(float64(c.config.BaseDelay) * math.Pow(2, float64(attempt)))

	// Cap at maximum delay
	if delay > c.config.MaxDelay {
		delay = c.config.MaxDelay
	}

	// Add jitter to avoid thundering herd
	jitter := time.Duration(float64(delay) * defaultJitterFactor * (rand.Float64() - 0.5) * 2)
	delay += jitter

	// Ensure delay is not negative
	if delay < 0 {
		delay = c.config.BaseDelay
	}

	return delay
}

// parseClaudeError converts various error types into ClaudeAPIError with retry information
func (c *ClaudeClient) parseClaudeError(err error) *ClaudeAPIError {
	if err == nil {
		return &ClaudeAPIError{
			Type:       "unknown",
			Message:    "unknown error",
			StatusCode: 0,
			Retryable:  false,
		}
	}

	// Check for context errors (timeout, cancellation)
	if errors.Is(err, context.DeadlineExceeded) {
		return &ClaudeAPIError{
			Type:       "timeout",
			Message:    "request timeout",
			StatusCode: 0,
			Retryable:  true,
		}
	}

	if errors.Is(err, context.Canceled) {
		return &ClaudeAPIError{
			Type:       "cancelled",
			Message:    "request cancelled",
			StatusCode: 0,
			Retryable:  false,
		}
	}

	// Convert error to string for pattern matching
	errStr := strings.ToLower(err.Error())

	// Check for rate limiting
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		return &ClaudeAPIError{
			Type:       "rate_limit_error",
			Message:    "API rate limit exceeded",
			StatusCode: 429,
			Retryable:  true,
		}
	}

	// Check for server errors (retryable)
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return &ClaudeAPIError{
			Type:       "server_error",
			Message:    "server error",
			StatusCode: 500,
			Retryable:  true,
		}
	}

	// Check for authentication errors (not retryable)
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "invalid api key") {
		return &ClaudeAPIError{
			Type:       "authentication_error",
			Message:    "invalid API key or unauthorized",
			StatusCode: 401,
			Retryable:  false,
		}
	}

	// Check for client errors (not retryable)
	if strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid request") {
		return &ClaudeAPIError{
			Type:       "invalid_request_error",
			Message:    "invalid request",
			StatusCode: 400,
			Retryable:  false,
		}
	}

	// Check for network errors (retryable)
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "timeout") || strings.Contains(errStr, "dns") {
		return &ClaudeAPIError{
			Type:       "network_error",
			Message:    "network or connection error",
			StatusCode: 0,
			Retryable:  true,
		}
	}

	// Default to non-retryable for unknown errors
	return &ClaudeAPIError{
		Type:       "api_error",
		Message:    err.Error(),
		StatusCode: 0,
		Retryable:  false,
	}
}

// formatWeatherContextFromExtracted creates structured context from already-extracted weather data
func (c *ClaudeClient) formatWeatherContextFromExtracted(todayData *TodayWeatherData) (string, error) {
	if todayData == nil {
		return "", fmt.Errorf("today data is nil")
	}

	// Build structured context for Claude
	var context strings.Builder

	// Header with location and time context
	// Use the already extracted location from todayData which has the correct city name
	cityLocation := todayData.Location

	context.WriteString(fmt.Sprintf("WEATHER DATA FOR %s\n", strings.ToUpper(cityLocation)))

	// Current conditions section
	context.WriteString("CURRENT CONDITIONS:\n")
	context.WriteString(fmt.Sprintf("- Today is %s\n", time.Now().Format("Monday, January 2 at 3:04 PM")))
	context.WriteString(fmt.Sprintf("- Temperature: %.0f%s\n", todayData.CurrentTemp, getTemperatureUnit(todayData.Units)))
	context.WriteString(fmt.Sprintf("- Conditions: %s\n", todayData.CurrentConditions))
	context.WriteString(fmt.Sprintf("- Wind: %s\n", todayData.WindConditions))
	context.WriteString("\n")

	// Today's forecast section
	context.WriteString("TODAY'S FORECAST:\n")
	context.WriteString(fmt.Sprintf("- High temperature: %.0f%s\n", todayData.TempHigh, getTemperatureUnit(todayData.Units)))
	context.WriteString(fmt.Sprintf("- Low temperature: %.0f%s\n", todayData.TempLow, getTemperatureUnit(todayData.Units)))
	context.WriteString(fmt.Sprintf("- Precipitation chance: %.0f%%\n", todayData.RainChance*100))

	// Temperature trend analysis
	tempRange := todayData.TempHigh - todayData.TempLow
	if tempRange > 20 {
		context.WriteString("- Temperature trend: Wide temperature range expected\n")
	} else if tempRange < 10 {
		context.WriteString("- Temperature trend: Stable temperatures throughout the day\n")
	} else {
		context.WriteString("- Temperature trend: Moderate temperature variation\n")
	}
	context.WriteString("\n")

	// Weather alerts and notable conditions
	if len(todayData.WeatherAlerts) > 0 {
		context.WriteString("WEATHER ALERTS:\n")
		for _, alert := range todayData.WeatherAlerts {
			context.WriteString(fmt.Sprintf("- %s\n", strings.Title(alert)))
		}
		context.WriteString("\n")
	}

	// Radio broadcast guidance with enhanced contextual information
	context.WriteString("BROADCAST NOTES:\n")

	// Add comprehensive contextual information
	now := time.Now()
	contextualNotes := c.generateContextualBroadcastNotes(todayData, now)
	for _, note := range contextualNotes {
		context.WriteString(fmt.Sprintf("- %s\n", note))
	}

	return context.String(), nil
}

// generateContextualBroadcastNotes creates comprehensive contextual broadcast notes
// based on current time, weather conditions, and seasonal/holiday awareness
func (c *ClaudeClient) generateContextualBroadcastNotes(todayData *TodayWeatherData, now time.Time) []string {
	var notes []string

	// Time of day context
	hour := now.Hour()
	timeOfDay := c.getTimeOfDay(now)
	notes = append(notes, fmt.Sprintf("Broadcast time of day: %s", timeOfDay))

	// Radio broadcast guidance
	notes = append(notes, "Use radio-friendly tone: conversational, clear, fun, and very engaging")

	// Day of week context
	weekday := now.Weekday()
	isWeekend := weekday == time.Saturday || weekday == time.Sunday
	if isWeekend {
		notes = append(notes, "Weekend broadcast: consider more relaxed, leisure-focused tone")
	} else {
		switch {
		case hour >= 6 && hour < 9:
			notes = append(notes, "Morning commute time: focus on travel conditions and daily planning")
		case hour >= 17 && hour < 19:
			notes = append(notes, "Evening commute time: emphasize evening and tomorrow's outlook")
		case hour >= 9 && hour < 17:
			notes = append(notes, "Business hours: consider more brief and informative")
		default:
			notes = append(notes, "Off-peak hours: consider more conversational, detailed approach")
		}
	}

	// Season and time of year context
	season := c.getSeason(now)
	notes = append(notes, fmt.Sprintf("Season: %s - tailor weather discussion for seasonal relevance", season))

	// Seasonal weather emphasis
	switch season {
	case "winter":
		if todayData.CurrentTemp < 32 { // Below freezing
			notes = append(notes, "Cold weather alert - emphasize warming layers, ice/snow conditions")
		}
		notes = append(notes, "Winter season - mention heating costs, winter activity fun, holiday travel if applicable")
	case "spring":
		notes = append(notes, "Spring season - focus on changing conditions, outdoor activities starting")
		if todayData.RainChance > 0.3 {
			notes = append(notes, "Spring rain - mention gardening, growth, renewal themes")
		}
	case "summer":
		if todayData.TempHigh > 85 || todayData.CurrentTemp > 85 {
			notes = append(notes, "Hot day - emphasize hydration, cooling, outdoor safety")
		}
		notes = append(notes, "Summer season - highlight outdoor events, vacation weather, beach/pool conditions, time-off from work")
	case "fall":
		notes = append(notes, "Fall season - mention changing leaves, back-to-school, harvest themes")
		if todayData.WindConditions != "" && strings.Contains(strings.ToLower(todayData.WindConditions), "wind") {
			notes = append(notes, "Fall winds - good opportunity for autumn weather imagery")
		}
	}

	// Holiday awareness
	if c.isHoliday(now) {
		holidayName := c.getHolidayName(now)
		if holidayName != "" {
			notes = append(notes, fmt.Sprintf("Holiday broadcast for %s - incorporate festive elements, travel considerations", holidayName))
		} else {
			notes = append(notes, "Holiday period - consider festive tone and travel weather impacts where appropriate")
		}
	}

	// Weather-specific contextual notes
	if todayData.RainChance > 0.8 {
		notes = append(notes, "Rain is very likely - emphasize rain gear, indoor alternatives like listening to music")
	} else if todayData.RainChance > 0.7 {
		notes = append(notes, "Rain is likely - emphasize rain gear, indoor alternatives like listening to music")
	} else if todayData.RainChance > 0.5 {
		notes = append(notes, "High rain probability - emphasize spinkles, indoor alternatives")
	} else if todayData.RainChance < 0.3 {
		notes = append(notes, "Rain is unlikely - good day for outdoor activities in a seasonal context")
	}

	// Temperature-specific guidance (regardless of season)
	hotThreshold := 85.0  // Fahrenheit
	coldThreshold := 32.0 // Fahrenheit
	coolThreshold := 50.0 // Fahrenheit

	if todayData.Units == "metric" {
		hotThreshold = 29.0  // ~85°F in Celsius
		coldThreshold = 0.0  // Freezing in Celsius
		coolThreshold = 10.0 // ~50°F in Celsius
	}

	if todayData.TempHigh > hotThreshold || todayData.CurrentTemp > hotThreshold {
		notes = append(notes, "Hot day - emphasize hydration, cooling, outdoor safety")
	} else if todayData.TempHigh < coldThreshold || todayData.CurrentTemp < coldThreshold {
		notes = append(notes, "Freezing temperatures - mention cold weather precautions")
	} else if todayData.TempHigh < coolThreshold {
		notes = append(notes, "Cool day - suggest layered clothing")
	}

	if todayData.CurrentTemp != 0 && todayData.TempHigh != 0 {
		tempRange := todayData.TempHigh - todayData.TempLow
		if tempRange > 25 {
			notes = append(notes, "Large temperature swing - mention layering clothes, changing conditions throughout day")
		}
	}

	// Wind conditions context
	if todayData.WindConditions != "" {
		windLower := strings.ToLower(todayData.WindConditions)
		if strings.Contains(windLower, "strong") || strings.Contains(windLower, "high") {
			notes = append(notes, "Strong winds - mention outdoor activity impacts, potential power/tree concerns")
		}
	}

	// Special weather alerts context
	if len(todayData.WeatherAlerts) > 0 {
		notes = append(notes, "Weather alerts active - maintain serious, informative tone while being reassuring")
	}

	// Location-specific notes (Seattle context from config)
	cityLocation := strings.ToLower(todayData.Location)
	if strings.Contains(cityLocation, "seattle") || strings.Contains(cityLocation, "47.6") {
		notes = append(notes, "Seattle area - reference local landmarks, ferry conditions, mountain visibility if clear")
		if strings.Contains(strings.ToLower(todayData.CurrentConditions), "clear") ||
			strings.Contains(strings.ToLower(todayData.CurrentConditions), "sunny") {
			notes = append(notes, "Clear Seattle weather - rare treat! Mention mountain views, outdoor opportunities")
		}
	}

	// Broadcast timing urgency
	if c.isUrgentTime(now) {
		notes = append(notes, "Peak listening time! Prioritize essential information, keep engaging and fun but concise")
	}

	return notes
}

// Helper functions for contextual broadcast notes

func (c *ClaudeClient) getTimeOfDay(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

func (c *ClaudeClient) getSeason(t time.Time) string {
	month := t.Month()
	switch {
	case month >= 3 && month <= 5:
		return "spring"
	case month >= 6 && month <= 8:
		return "summer"
	case month >= 9 && month <= 11:
		return "fall"
	default:
		return "winter"
	}
}

func (c *ClaudeClient) isHoliday(t time.Time) bool {
	return c.checkUSHolidays(t) || c.checkInternationalHolidays(t)
}

func (c *ClaudeClient) getHolidayName(t time.Time) string {
	month := t.Month()
	day := t.Day()

	// Major fixed holidays
	switch {
	case month == time.January && day == 1:
		return "New Year's Day"
	case month == time.February && day == 14:
		return "Valentine's Day"
	case month == time.March && day == 17:
		return "St. Patrick's Day"
	case month == time.July && day == 4:
		return "Independence Day"
	case month == time.October && day == 31:
		return "Halloween"
	case month == time.November && day == 11:
		return "Veterans Day"
	case month == time.December && day == 25:
		return "Christmas Day"
	case month == time.December && day == 31:
		return "New Year's Eve"
	}

	// Variable holidays (simplified check)
	if month == time.November && c.isNthWeekdayOfMonth(t, 4, time.Thursday) {
		return "Thanksgiving"
	}
	if month == time.May && c.isLastWeekdayOfMonth(t, time.Monday) {
		return "Memorial Day"
	}
	if month == time.September && c.isNthWeekdayOfMonth(t, 1, time.Monday) {
		return "Labor Day"
	}

	return ""
}

func (c *ClaudeClient) checkUSHolidays(t time.Time) bool {
	month := t.Month()
	day := t.Day()

	// Fixed holidays
	fixedHolidays := map[time.Month][]int{
		time.January:  {1},      // New Year's Day
		time.February: {14},     // Valentine's Day
		time.March:    {17},     // St. Patrick's Day
		time.July:     {4},      // Independence Day
		time.October:  {31},     // Halloween
		time.November: {11},     // Veterans Day
		time.December: {25, 31}, // Christmas Day, New Year's Eve
	}

	if days, exists := fixedHolidays[month]; exists {
		for _, holidayDay := range days {
			if day == holidayDay {
				return true
			}
		}
	}

	// Variable holidays
	switch month {
	case time.January:
		return c.isNthWeekdayOfMonth(t, 3, time.Monday) // MLK Day
	case time.February:
		return c.isNthWeekdayOfMonth(t, 3, time.Monday) // Presidents' Day
	case time.May:
		return c.isLastWeekdayOfMonth(t, time.Monday) || c.isNthWeekdayOfMonth(t, 2, time.Sunday) // Memorial Day, Mother's Day
	case time.June:
		return c.isNthWeekdayOfMonth(t, 3, time.Sunday) // Father's Day
	case time.September:
		return c.isNthWeekdayOfMonth(t, 1, time.Monday) // Labor Day
	case time.October:
		return c.isNthWeekdayOfMonth(t, 2, time.Monday) // Columbus Day
	case time.November:
		return c.isNthWeekdayOfMonth(t, 4, time.Thursday) // Thanksgiving
	}

	return false
}

func (c *ClaudeClient) checkInternationalHolidays(t time.Time) bool {
	month := t.Month()
	day := t.Day()

	internationalHolidays := map[time.Month][]int{
		time.January:  {1},  // New Year's Day
		time.February: {14}, // Valentine's Day
		time.March:    {8},  // International Women's Day
		time.April:    {22}, // Earth Day
		time.May:      {1},  // International Workers' Day
		time.October:  {31}, // Halloween
		time.December: {25}, // Christmas Day
	}

	if days, exists := internationalHolidays[month]; exists {
		for _, holidayDay := range days {
			if day == holidayDay {
				return true
			}
		}
	}

	return false
}

func (c *ClaudeClient) isNthWeekdayOfMonth(t time.Time, n int, weekday time.Weekday) bool {
	if t.Weekday() != weekday {
		return false
	}

	firstOfMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	firstWeekday := firstOfMonth
	for firstWeekday.Weekday() != weekday {
		firstWeekday = firstWeekday.AddDate(0, 0, 1)
	}

	nthWeekday := firstWeekday.AddDate(0, 0, (n-1)*7)
	return t.Day() == nthWeekday.Day() && t.Month() == nthWeekday.Month()
}

func (c *ClaudeClient) isLastWeekdayOfMonth(t time.Time, weekday time.Weekday) bool {
	if t.Weekday() != weekday {
		return false
	}

	nextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	lastOfMonth := nextMonth.AddDate(0, 0, -1)

	lastWeekday := lastOfMonth
	for lastWeekday.Weekday() != weekday {
		lastWeekday = lastWeekday.AddDate(0, 0, -1)
	}

	return t.Day() == lastWeekday.Day()
}

func (c *ClaudeClient) isUrgentTime(t time.Time) bool {
	hour := t.Hour()
	weekday := t.Weekday()

	// Business hours on weekdays are less urgent than off-hours
	if weekday >= time.Monday && weekday <= time.Friday {
		// Morning/evening commute times are urgent
		if (hour >= 6 && hour < 9) || (hour >= 17 && hour < 19) {
			return true
		}
	}

	return false
}

// extractWeatherVariables converts weather forecast data into template variables

// validateGeneratedScript ensures the script meets radio broadcast requirements
func (c *ClaudeClient) validateGeneratedScript(script string) error {
	// AIDEV-NOTE: Enhanced validation for radio broadcast requirements
	script = strings.TrimSpace(script)

	if script == "" {
		return fmt.Errorf("generated script is empty")
	}

	// Check minimum length (at least 50 characters for a meaningful report)
	if len(script) < 50 {
		return fmt.Errorf("generated script is too short (%d characters)", len(script))
	}

	// Check maximum length (radio reports should be concise, ~2-3 minutes at 150 WPM = ~450-750 words)
	if len(script) > 5000 {
		return fmt.Errorf("generated script is too long (%d characters)", len(script))
	}

	// Word count validation for radio broadcast timing
	wordCount := len(strings.Fields(script))
	if wordCount < 15 {
		return fmt.Errorf("generated script has too few words (%d words, minimum 15)", wordCount)
	}
	if wordCount > 800 {
		return fmt.Errorf("generated script has too many words (%d words, maximum 800 for ~5 minute broadcast)", wordCount)
	}

	// AIDEV-NOTE: Radio broadcast content validation
	scriptLower := strings.ToLower(script)

	// Ensure script contains weather-related content
	weatherKeywords := []string{"temperature", "weather", "degrees", "rain", "wind", "sunny", "cloudy", "forecast"}
	hasWeatherContent := false
	for _, keyword := range weatherKeywords {
		if strings.Contains(scriptLower, keyword) {
			hasWeatherContent = true
			break
		}
	}
	if !hasWeatherContent {
		return fmt.Errorf("generated script appears to lack weather-related content")
	}

	return nil
}

// getTemperatureUnit returns the appropriate temperature unit symbol based on units
func getTemperatureUnit(units string) string {
	switch strings.ToLower(units) {
	case "metric":
		return "°C"
	case "imperial":
		return "°F"
	case "kelvin":
		return " K"
	default:
		return ""
	}
}

// logPromptToFile logs the full Claude prompt and weather data to results.log
func (c *ClaudeClient) logPromptToFile(request WeatherReportRequest, weatherContext string, messageReq anthropic.MessageNewParams) error {
	// Determine output directory from the request context
	outputDir := request.OutputPath
	if outputDir == "" {
		outputDir = "." // Default to current directory
	}

	logFilePath := filepath.Join(outputDir, "results.log")

	// Create log entry with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")
	logEntry := fmt.Sprintf(`
=== CLAUDE WEATHER REPORT GENERATION ===
Timestamp: %s
Location: %s

=== WEATHER DATA (System Context) ===
%s

=== USER PROMPT TEMPLATE ===
%s

=== CLAUDE API PARAMETERS ===
Model: %s
Max Tokens: %d
Temperature: %.2f

=== END LOG ENTRY ===

`, timestamp, request.Location, weatherContext, request.PromptTemplate,
		string(messageReq.Model), messageReq.MaxTokens, messageReq.Temperature.Value)

	// Overwrite log file each time
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open results.log: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to results.log: %w", err)
	}

	logger.LogWithFields(logger.DebugLevel, "Claude prompt logged to file", map[string]any{
		"log_file": logFilePath,
		"location": request.Location,
	})

	return nil
}

// appendScriptToLog appends the generated Claude script to the results.log file
func (c *ClaudeClient) appendScriptToLog(request WeatherReportRequest, script string) error {
	// Determine output directory from the request context
	outputDir := request.OutputPath
	if outputDir == "" {
		outputDir = "." // Default to current directory
	}
	logFilePath := filepath.Join(outputDir, "results.log")

	// Read the existing log content
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to read existing results.log: %w", err)
	}

	// Find the "=== END LOG ENTRY ===" line and insert the script before it
	existingContent := string(content)
	endMarker := "=== END LOG ENTRY ==="

	// Check if the end marker exists
	if !strings.Contains(existingContent, endMarker) {
		return fmt.Errorf("end log entry marker not found in results.log")
	}

	// Create the script section
	scriptSection := fmt.Sprintf(`
=== CLAUDE WEATHER SCRIPT ===
%s

`, script)

	// Insert the script section before the end marker
	modifiedContent := strings.Replace(existingContent, endMarker, scriptSection+endMarker, 1)

	// Write the modified content back to the file
	if err := os.WriteFile(logFilePath, []byte(modifiedContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated results.log: %w", err)
	}

	logger.LogWithFields(logger.DebugLevel, "Claude script appended to log file", map[string]any{
		"log_file":      logFilePath,
		"script_length": len(script),
	})

	return nil
}
