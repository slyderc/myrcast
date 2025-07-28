package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/haguro/elevenlabs-go"
	"myrcast/internal/logger"
)

const (
	// Default ElevenLabs configuration
	defaultElevenLabsTimeout = 30 * time.Second
	defaultJitterFactorEL    = 0.1

	// Audio conversion settings for radio broadcast
	targetSampleRate = 44100 // 44.1 kHz for radio
	targetBitDepth   = 16    // 16-bit audio
	targetChannels   = 1     // Mono for voice
)

// ElevenLabsClient handles ElevenLabs API interactions with retry logic and rate limiting
type ElevenLabsClient struct {
	client      *elevenlabs.Client
	config      ElevenLabsConfig
	rateLimiter *ElevenLabsRateLimiter
}

// ElevenLabsConfig contains configuration for ElevenLabs API client
type ElevenLabsConfig struct {
	APIKey      string
	VoiceID     string
	Model       string
	Stability   float64
	Similarity  float64
	Style       float64
	Speed       float64
	Format      string
	Timeout     time.Duration
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RateLimit   int // requests per minute
}

// ElevenLabsRateLimiter handles rate limiting for ElevenLabs API requests
type ElevenLabsRateLimiter struct {
	requests    []time.Time
	maxRequests int
	window      time.Duration
}

// CustomVoiceSettings extends the SDK VoiceSettings to include speed parameter
// AIDEV-NOTE: Using custom struct since the SDK doesn't support speed parameter yet
type CustomVoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
	SpeakerBoost    *bool    `json:"use_speaker_boost,omitempty"`
}

// CustomTextToSpeechRequest for direct API calls with speed support
type CustomTextToSpeechRequest struct {
	Text          string                `json:"text"`
	ModelID       string                `json:"model_id,omitempty"`
	VoiceSettings *CustomVoiceSettings  `json:"voice_settings,omitempty"`
}

// NewElevenLabsRateLimiter creates a new rate limiter for ElevenLabs API
func NewElevenLabsRateLimiter(requestsPerMinute int) *ElevenLabsRateLimiter {
	return &ElevenLabsRateLimiter{
		requests:    make([]time.Time, 0),
		maxRequests: requestsPerMinute,
		window:      time.Minute,
	}
}

// Wait blocks until a request can be made according to rate limits
func (rl *ElevenLabsRateLimiter) Wait(ctx context.Context) error {
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
		logger.LogWithFields(logger.InfoLevel, "ElevenLabs API rate limit reached, waiting", map[string]any{
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

// ElevenLabsAPIError represents errors from the ElevenLabs API
type ElevenLabsAPIError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StatusCode int
	Retryable  bool
}

func (e *ElevenLabsAPIError) Error() string {
	return fmt.Sprintf("ElevenLabs API error (status %d, type %s): %s", e.StatusCode, e.Type, e.Message)
}

// IsRetryable returns true if this error indicates a retryable condition
func (e *ElevenLabsAPIError) IsRetryable() bool {
	return e.Retryable
}

// NewElevenLabsClient creates a new ElevenLabs API client with the provided configuration
func NewElevenLabsClient(config ElevenLabsConfig) (*ElevenLabsClient, error) {
	// AIDEV-NOTE: Validate API key before creating client to fail fast
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("ElevenLabs API key is required")
	}

	// Apply defaults to configuration
	if config.VoiceID == "" {
		config.VoiceID = "pNInz6obpgDQGcFmaJgB" // Default Adam voice
	}
	if config.Model == "" {
		config.Model = "eleven_multilingual_v1"
	}
	if config.Stability <= 0 {
		config.Stability = 0.5
	}
	if config.Similarity <= 0 {
		config.Similarity = 0.8
	}
	if config.Style < 0 {
		config.Style = 0.0
	}
	if config.Speed <= 0 {
		config.Speed = 1.0
	}
	if config.Format == "" {
		config.Format = "mp3_44100_128"
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultElevenLabsTimeout
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.BaseDelay <= 0 {
		config.BaseDelay = 1 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.RateLimit <= 0 {
		config.RateLimit = 20 // Conservative rate limit
	}

	// Create ElevenLabs client with context and timeout
	client := elevenlabs.NewClient(context.Background(), config.APIKey, config.Timeout)

	// Create rate limiter
	rateLimiter := NewElevenLabsRateLimiter(config.RateLimit)

	return &ElevenLabsClient{
		client:      client,
		config:      config,
		rateLimiter: rateLimiter,
	}, nil
}

// TextToSpeechRequest contains the request data for generating speech
type TextToSpeechRequest struct {
	Text     string // Text to convert to speech
	VoiceID  string // Override default voice ID (optional)
	OutputDir string // Directory to save the generated audio file
	FileName string // Name for the output file (without extension)
}

// TextToSpeechResponse contains the generated speech audio
type TextToSpeechResponse struct {
	AudioFilePath string    // Path to the generated WAV file
	OriginalMP3   string    // Path to the original MP3 file from ElevenLabs
	DurationMs    int       // Duration in milliseconds
	VoiceUsed     string    // Voice ID that was used
	GeneratedAt   time.Time // Timestamp of generation
}

// GenerateTextToSpeech converts text to speech using ElevenLabs with retry logic and rate limiting
func (c *ElevenLabsClient) GenerateTextToSpeech(ctx context.Context, request TextToSpeechRequest) (*TextToSpeechResponse, error) {
	// AIDEV-NOTE: Enhanced with retry logic, rate limiting, and audio format conversion
	complete := logger.LogOperationStart("elevenlabs_text_to_speech_with_retry", map[string]any{
		"voice_id":     c.getVoiceID(request.VoiceID),
		"model":        c.config.Model,
		"stability":    c.config.Stability,
		"similarity":   c.config.Similarity,
		"style":        c.config.Style,
		"speed":        c.config.Speed,
		"text_length":  len(request.Text),
		"max_retries":  c.config.MaxRetries,
	})

	// Validate input
	if err := c.validateTextToSpeechRequest(request); err != nil {
		complete(fmt.Errorf("invalid request: %w", err))
		return nil, fmt.Errorf("invalid text-to-speech request: %w", err)
	}

	// Determine voice ID to use
	voiceID := c.getVoiceID(request.VoiceID)

	// Create custom ElevenLabs request matching working curl command structure
	// AIDEV-NOTE: Matching exact JSON structure from working curl command
	speed := c.config.Speed
	ttsReq := CustomTextToSpeechRequest{
		Text:    request.Text,
		ModelID: c.config.Model,
		VoiceSettings: &CustomVoiceSettings{
			Speed: &speed, // Only speed parameter like the working curl
		},
	}

	// Execute request with retry logic using custom API call
	audioData, err := c.executeCustomTextToSpeechWithRetry(ctx, voiceID, ttsReq)
	if err != nil {
		complete(fmt.Errorf("ElevenLabs API request failed after retries: %w", err))
		return nil, err
	}

	// Save MP3 file (no conversion needed - Myriad supports MP3)
	mp3FilePath, err := c.saveMP3Audio(audioData, request.OutputDir, request.FileName)
	if err != nil {
		complete(fmt.Errorf("failed to save MP3 audio: %w", err))
		return nil, fmt.Errorf("failed to save MP3 audio: %w", err)
	}

	// Calculate audio duration from MP3 (simplified - no WAV conversion needed)
	duration, err := c.calculateMP3Duration(mp3FilePath)
	if err != nil {
		logger.LogWithFields(logger.WarnLevel, "Failed to calculate audio duration", map[string]any{
			"error":    err.Error(),
			"mp3_file": mp3FilePath,
		})
		duration = 0 // Set to 0 if we can't calculate
	}

	complete(nil)

	return &TextToSpeechResponse{
		AudioFilePath: mp3FilePath, // Return MP3 directly
		OriginalMP3:   mp3FilePath, // Same file now
		DurationMs:    duration,
		VoiceUsed:     voiceID,
		GeneratedAt:   time.Now(),
	}, nil
}


// executeCustomTextToSpeechWithRetry executes a custom TTS request with speed support
func (c *ElevenLabsClient) executeCustomTextToSpeechWithRetry(ctx context.Context, voiceID string, ttsReq CustomTextToSpeechRequest) ([]byte, error) {
	var lastErr error
	baseURL := "https://api.elevenlabs.io/v1/text-to-speech/" + voiceID + "?output_format=" + c.config.Format

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Apply rate limiting before each request
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter cancelled: %w", err)
		}

		// Log attempt
		if attempt > 0 {
			logger.LogWithFields(logger.InfoLevel, "Retrying ElevenLabs custom API request", map[string]any{
				"attempt":     attempt + 1,
				"max_retries": c.config.MaxRetries + 1,
				"voice_id":    voiceID,
				"speed":       ttsReq.VoiceSettings.Speed,
			})
		}

		// Marshal request to JSON
		jsonData, err := json.Marshal(ttsReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}



		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("xi-api-key", c.config.APIKey)

		// Execute the HTTP request
		client := &http.Client{Timeout: c.config.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			elevenLabsErr := c.parseElevenLabsError(err)

			// Check if this is the last attempt or if error is not retryable
			if attempt == c.config.MaxRetries || !elevenLabsErr.IsRetryable() {
				if !elevenLabsErr.IsRetryable() {
					logger.LogWithFields(logger.ErrorLevel, "Non-retryable ElevenLabs custom API error", map[string]any{
						"error":    err.Error(),
						"attempt":  attempt + 1,
						"voice_id": voiceID,
					})
					return nil, elevenLabsErr
				}
				break // Exit retry loop on last attempt
			}

			// Calculate delay for next retry
			delay := c.calculateRetryDelay(attempt)

			logger.LogWithFields(logger.WarnLevel, "ElevenLabs custom API request failed, retrying", map[string]any{
				"error":        err.Error(),
				"attempt":      attempt + 1,
				"next_attempt": attempt + 2,
				"delay_ms":     delay.Milliseconds(),
				"voice_id":     voiceID,
			})

			// Wait before next retry
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("ElevenLabs API returned status %d: %s", resp.StatusCode, string(bodyBytes))
			
			elevenLabsErr := &ElevenLabsAPIError{
				Type:       "api_error",
				Message:    string(bodyBytes),
				StatusCode: resp.StatusCode,
				Retryable:  resp.StatusCode >= 500 || resp.StatusCode == 429,
			}

			if attempt == c.config.MaxRetries || !elevenLabsErr.IsRetryable() {
				if !elevenLabsErr.IsRetryable() {
					logger.LogWithFields(logger.ErrorLevel, "Non-retryable ElevenLabs custom API error", map[string]any{
						"status_code": resp.StatusCode,
						"response":    string(bodyBytes),
						"attempt":     attempt + 1,
						"voice_id":    voiceID,
					})
					return nil, elevenLabsErr
				}
				break
			}

			// Calculate delay for next retry
			delay := c.calculateRetryDelay(attempt)

			logger.LogWithFields(logger.WarnLevel, "ElevenLabs custom API request failed, retrying", map[string]any{
				"status_code":  resp.StatusCode,
				"response":     string(bodyBytes),
				"attempt":      attempt + 1,
				"next_attempt": attempt + 2,
				"delay_ms":     delay.Milliseconds(),
				"voice_id":     voiceID,
			})

			// Wait before next retry
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Read response body
		audioData, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Success!
		if attempt > 0 {
			logger.LogWithFields(logger.InfoLevel, "ElevenLabs custom API request succeeded after retries", map[string]any{
				"successful_attempt": attempt + 1,
				"total_attempts":     attempt + 1,
				"voice_id":           voiceID,
				"speed":              ttsReq.VoiceSettings.Speed,
			})
		}
		return audioData, nil
	}

	// All retries exhausted
	elevenLabsErr := c.parseElevenLabsError(lastErr)
	logger.LogWithFields(logger.ErrorLevel, "ElevenLabs custom API request failed after all retries", map[string]any{
		"total_attempts": c.config.MaxRetries + 1,
		"final_error":    lastErr.Error(),
		"voice_id":       voiceID,
	})

	return nil, fmt.Errorf("ElevenLabs custom API request failed after %d attempts: %w", c.config.MaxRetries+1, elevenLabsErr)
}

// calculateRetryDelay calculates the delay for the next retry attempt using exponential backoff with jitter
func (c *ElevenLabsClient) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(float64(c.config.BaseDelay) * math.Pow(2, float64(attempt)))

	// Cap at maximum delay
	if delay > c.config.MaxDelay {
		delay = c.config.MaxDelay
	}

	// Add jitter to avoid thundering herd
	jitter := time.Duration(float64(delay) * defaultJitterFactorEL * (rand.Float64() - 0.5) * 2)
	delay += jitter

	// Ensure delay is not negative
	if delay < 0 {
		delay = c.config.BaseDelay
	}

	return delay
}

// parseElevenLabsError converts various error types into ElevenLabsAPIError with retry information
func (c *ElevenLabsClient) parseElevenLabsError(err error) *ElevenLabsAPIError {
	if err == nil {
		return &ElevenLabsAPIError{
			Type:       "unknown",
			Message:    "unknown error",
			StatusCode: 0,
			Retryable:  false,
		}
	}

	// Check for context errors (timeout, cancellation)
	if errors.Is(err, context.DeadlineExceeded) {
		return &ElevenLabsAPIError{
			Type:       "timeout",
			Message:    "request timeout",
			StatusCode: 0,
			Retryable:  true,
		}
	}

	if errors.Is(err, context.Canceled) {
		return &ElevenLabsAPIError{
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
		return &ElevenLabsAPIError{
			Type:       "rate_limit_error",
			Message:    "API rate limit exceeded",
			StatusCode: 429,
			Retryable:  true,
		}
	}

	// Check for server errors (retryable)
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return &ElevenLabsAPIError{
			Type:       "server_error",
			Message:    "server error",
			StatusCode: 500,
			Retryable:  true,
		}
	}

	// Check for authentication errors (not retryable)
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "invalid api key") {
		return &ElevenLabsAPIError{
			Type:       "authentication_error",
			Message:    "invalid API key or unauthorized",
			StatusCode: 401,
			Retryable:  false,
		}
	}

	// Check for client errors (not retryable)
	if strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid request") {
		return &ElevenLabsAPIError{
			Type:       "invalid_request_error",
			Message:    "invalid request",
			StatusCode: 400,
			Retryable:  false,
		}
	}

	// Check for network errors (retryable)
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "timeout") || strings.Contains(errStr, "dns") {
		return &ElevenLabsAPIError{
			Type:       "network_error",
			Message:    "network or connection error",
			StatusCode: 0,
			Retryable:  true,
		}
	}

	// Default to non-retryable for unknown errors
	return &ElevenLabsAPIError{
		Type:       "api_error",
		Message:    err.Error(),
		StatusCode: 0,
		Retryable:  false,
	}
}

// Helper functions

// getVoiceID returns the voice ID to use, preferring override over default
func (c *ElevenLabsClient) getVoiceID(override string) string {
	if override != "" {
		return override
	}
	return c.config.VoiceID
}

// validateTextToSpeechRequest validates the input request
func (c *ElevenLabsClient) validateTextToSpeechRequest(request TextToSpeechRequest) error {
	// Check text content
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return fmt.Errorf("text content is required")
	}

	// Check text length (ElevenLabs has limits)
	if len(text) < 10 {
		return fmt.Errorf("text content too short (%d characters), minimum 10 characters", len(text))
	}

	if len(text) > 5000 {
		return fmt.Errorf("text content too long (%d characters), maximum 5000 characters", len(text))
	}

	// Check output directory
	if strings.TrimSpace(request.OutputDir) == "" {
		return fmt.Errorf("output directory is required")
	}

	// Check filename
	if strings.TrimSpace(request.FileName) == "" {
		return fmt.Errorf("output filename is required")
	}

	return nil
}

// saveMP3Audio saves the MP3 audio data to a file
func (c *ElevenLabsClient) saveMP3Audio(audioData []byte, outputDir, fileName string) (string, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create MP3 file path
	mp3FilePath := filepath.Join(outputDir, fileName+".mp3")

	// Write MP3 data to file
	if err := os.WriteFile(mp3FilePath, audioData, 0644); err != nil {
		return "", fmt.Errorf("failed to write MP3 file: %w", err)
	}

	logger.LogWithFields(logger.InfoLevel, "MP3 audio file saved", map[string]any{
		"file_path": mp3FilePath,
		"file_size": len(audioData),
	})

	return mp3FilePath, nil
}

// calculateMP3Duration calculates the duration of an MP3 file in milliseconds
// AIDEV-NOTE: Simple duration calculation using file size estimation for MP3
func (c *ElevenLabsClient) calculateMP3Duration(mp3FilePath string) (int, error) {
	// Get file size
	fileInfo, err := os.Stat(mp3FilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get MP3 file info: %w", err)
	}
	
	// Simple estimation: MP3 at 128kbps ≈ 16KB per second
	// This is rough but sufficient for logging purposes
	fileSizeKB := fileInfo.Size() / 1024
	estimatedSeconds := fileSizeKB / 16
	estimatedMs := int(estimatedSeconds * 1000)
	
	logger.LogWithFields(logger.InfoLevel, "MP3 duration estimated", map[string]any{
		"file_path":       mp3FilePath,
		"file_size_kb":    fileSizeKB,
		"duration_ms":     estimatedMs,
	})
	
	return estimatedMs, nil
}

