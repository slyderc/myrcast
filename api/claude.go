package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
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
		logger.LogWithFields(logger.InfoLevel, "Claude API rate limit reached, waiting", map[string]any{
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

	// Apply defaults to configuration
	if config.Model == "" {
		config.Model = defaultModel
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultMaxTokens
	}
	if config.Temperature <= 0 {
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
	WeatherData    *ForecastResponse // Weather data from OpenWeather API
	PromptTemplate string            // Template with variable placeholders
	Location       string            // Location name for the report
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

	// Format weather data for Claude context
	weatherContext, err := c.formatWeatherContext(request.WeatherData, request.Location)
	if err != nil {
		complete(fmt.Errorf("failed to format weather context: %w", err))
		return nil, fmt.Errorf("failed to format weather context: %w", err)
	}

	// Extract weather data for template variables
	weatherData, err := c.extractWeatherVariables(request.WeatherData, request.Location)
	if err != nil {
		complete(fmt.Errorf("failed to extract weather variables: %w", err))
		return nil, fmt.Errorf("failed to extract weather variables: %w", err)
	}

	// Substitute variables in the prompt template
	prompt := c.substituteTemplateVariables(request.PromptTemplate, weatherData)

	// Build the message request
	messageReq := anthropic.MessageNewParams{
		Model:       anthropic.Model(c.config.Model),
		MaxTokens:   int64(c.config.MaxTokens),
		Temperature: anthropic.Float(c.config.Temperature),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(prompt),
			),
		},
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: weatherContext,
			},
		},
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
			logger.LogWithFields(logger.InfoLevel, "Retrying Claude API request", map[string]any{
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
			logger.LogWithFields(logger.InfoLevel, "Claude API request succeeded after retries", map[string]any{
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

// formatWeatherContext creates structured context from weather data
func (c *ClaudeClient) formatWeatherContext(weather *ForecastResponse, location string) (string, error) {
	if weather == nil {
		return "", fmt.Errorf("weather data is nil")
	}

	// Extract today's weather data
	weatherClient := &WeatherClient{} // We only need the extraction method
	todayData, err := weatherClient.ExtractTodayWeather(weather)
	if err != nil {
		return "", fmt.Errorf("failed to extract today's weather: %w", err)
	}

	// Build structured context for Claude
	var context strings.Builder

	// Header with location and time context
	context.WriteString(fmt.Sprintf("WEATHER DATA FOR %s\n", strings.ToUpper(location)))
	context.WriteString(fmt.Sprintf("Report generated: %s\n", time.Now().Format("Monday, January 2, 2006 at 3:04 PM")))
	context.WriteString(fmt.Sprintf("Data source: %s, %s\n\n", weather.City.Name, weather.City.Country))

	// Current conditions section
	context.WriteString("CURRENT CONDITIONS:\n")
	context.WriteString(fmt.Sprintf("- Temperature: %s\n", formatTemperature(todayData.CurrentTemp, todayData.Units)))
	context.WriteString(fmt.Sprintf("- Conditions: %s\n", todayData.CurrentConditions))
	context.WriteString(fmt.Sprintf("- Wind: %s\n", todayData.WindConditions))
	context.WriteString("\n")

	// Today's forecast section
	context.WriteString("TODAY'S FORECAST:\n")
	context.WriteString(fmt.Sprintf("- High temperature: %s\n", formatTemperature(todayData.TempHigh, todayData.Units)))
	context.WriteString(fmt.Sprintf("- Low temperature: %s\n", formatTemperature(todayData.TempLow, todayData.Units)))
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

	// Radio broadcast guidance
	context.WriteString("BROADCAST NOTES:\n")
	context.WriteString("- This weather data should be presented in a conversational, radio-friendly tone\n")
	context.WriteString("- Focus on information most relevant to listeners' daily activities\n")
	context.WriteString("- Include timing for any weather changes throughout the day\n")

	// Rain guidance (todayData.RainChance is already 0-1, convert to percentage)
	rainChancePercent := todayData.RainChance * 100
	if rainChancePercent > 70 {
		context.WriteString("- Rain is likely - mention umbrella/indoor activities\n")
	} else if rainChancePercent > 30 {
		context.WriteString("- Rain is possible - mention it may be worth watching the sky\n")
	} else {
		context.WriteString("- Rain is unlikely - good day for outdoor activities\n")
	}

	// Temperature guidance
	if todayData.Units == "imperial" {
		if todayData.TempHigh > 85 {
			context.WriteString("- Hot day - mention staying hydrated and cool\n")
		} else if todayData.TempLow < 32 {
			context.WriteString("- Freezing temperatures - mention bundling up and potential ice\n")
		} else if todayData.TempHigh < 50 {
			context.WriteString("- Cool day - mention layering clothes\n")
		}
	} else if todayData.Units == "metric" {
		if todayData.TempHigh > 29 {
			context.WriteString("- Hot day - mention staying hydrated and cool\n")
		} else if todayData.TempLow < 0 {
			context.WriteString("- Freezing temperatures - mention bundling up and potential ice\n")
		} else if todayData.TempHigh < 10 {
			context.WriteString("- Cool day - mention layering clothes\n")
		}
	}

	return context.String(), nil
}

// extractWeatherVariables converts weather forecast data into template variables
func (c *ClaudeClient) extractWeatherVariables(forecast *ForecastResponse, location string) (map[string]string, error) {
	if forecast == nil {
		return nil, fmt.Errorf("forecast data is nil")
	}

	// Create a weather client to extract today's data
	weatherClient := &WeatherClient{} // We only need the extraction method
	todayData, err := weatherClient.ExtractTodayWeather(forecast)
	if err != nil {
		return nil, fmt.Errorf("failed to extract today's weather: %w", err)
	}

	// Create variables map with all available template variables
	variables := map[string]string{
		// Location information
		"location": location,
		"city":     forecast.City.Name,
		"country":  forecast.City.Country,

		// Temperature data
		"current_temp": formatTemperature(todayData.CurrentTemp, todayData.Units),
		"temp_high":    formatTemperature(todayData.TempHigh, todayData.Units),
		"temp_low":     formatTemperature(todayData.TempLow, todayData.Units),

		// Weather conditions
		"current_conditions": todayData.CurrentConditions,
		"wind_conditions":    todayData.WindConditions,

		// Precipitation
		"rain_chance": formatPercentage(todayData.RainChance),

		// System information
		"units": todayData.Units,

		// Time information
		"date":         time.Now().Format("January 2"),
		"dow":          time.Now().Format("Monday"),
		"time":         time.Now().Format("3:04 PM"),
		"last_updated": todayData.LastUpdated.Format("3:04 PM"),
	}

	// Add weather alerts as a comma-separated string
	if len(todayData.WeatherAlerts) > 0 {
		variables["weather_alerts"] = strings.Join(todayData.WeatherAlerts, ", ")
	} else {
		variables["weather_alerts"] = "none"
	}

	return variables, nil
}

// substituteTemplateVariables replaces template variables with actual values
func (c *ClaudeClient) substituteTemplateVariables(template string, variables map[string]string) string {
	// AIDEV-NOTE: Simple template format using {{variable}} only
	re := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	result := re.ReplaceAllStringFunc(template, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		varName := submatch[1]
		if value, exists := variables[varName]; exists {
			return value
		}
		// Log warning for missing variable but don't fail
		logger.LogOperationStart("template_substitution_warning", map[string]any{
			"missing_variable": varName,
			"available_vars":   getVariableNames(variables),
		})
		return fmt.Sprintf("[missing:%s]", varName) // Placeholder for missing variables
	})

	return result
}

// Helper functions for formatting weather data

// formatTemperature formats temperature value with appropriate unit symbol
func formatTemperature(temp float64, units string) string {
	switch strings.ToLower(units) {
	case "metric":
		return fmt.Sprintf("%.0f°C", temp)
	case "imperial":
		return fmt.Sprintf("%.0f°F", temp)
	case "kelvin":
		return fmt.Sprintf("%.0f K", temp)
	default:
		return fmt.Sprintf("%.1f", temp)
	}
}

// formatPercentage formats a probability value (0-1) as a percentage
func formatPercentage(prob float64) string {
	return fmt.Sprintf("%.0f%%", prob*100)
}

// getVariableNames returns a slice of available variable names for logging
func getVariableNames(variables map[string]string) []string {
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	return names
}

// validateGeneratedScript ensures the script meets requirements
func (c *ClaudeClient) validateGeneratedScript(script string) error {
	// AIDEV-NOTE: Basic validation - ensure script is not empty and reasonable length
	script = strings.TrimSpace(script)

	if script == "" {
		return fmt.Errorf("generated script is empty")
	}

	// Check minimum length (at least 50 characters for a meaningful report)
	if len(script) < 50 {
		return fmt.Errorf("generated script is too short (%d characters)", len(script))
	}

	// Check maximum length (radio reports should be concise)
	if len(script) > 5000 {
		return fmt.Errorf("generated script is too long (%d characters)", len(script))
	}

	return nil
}
