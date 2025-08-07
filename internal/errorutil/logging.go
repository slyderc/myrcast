package errorutil

import (
	"fmt"
	"log/slog"
	"time"
)

// LogAndWrap logs an error with structured context and returns a wrapped error
// This consolidates the common pattern of structured logging followed by error wrapping
func LogAndWrap(logger *slog.Logger, operation string, err error, attrs ...slog.Attr) error {
	if logger == nil || err == nil {
		return err
	}

	// Build log attributes starting with the error
	logAttrs := []slog.Attr{
		slog.String("error", err.Error()),
	}
	logAttrs = append(logAttrs, attrs...)

	// Convert to interface slice for logger
	anyAttrs := make([]any, len(logAttrs))
	for i, attr := range logAttrs {
		anyAttrs[i] = attr
	}

	logger.Error(operation+" failed", anyAttrs...)
	return fmt.Errorf("%s: %w", operation, err)
}

// LogWarning logs a non-fatal error as warning without wrapping
// Used for recoverable errors that should be logged but don't stop processing
func LogWarning(logger *slog.Logger, operation string, err error, attrs ...slog.Attr) {
	if logger == nil || err == nil {
		return
	}

	logAttrs := []slog.Attr{
		slog.String("error", err.Error()),
	}
	logAttrs = append(logAttrs, attrs...)

	anyAttrs := make([]any, len(logAttrs))
	for i, attr := range logAttrs {
		anyAttrs[i] = attr
	}

	logger.Warn("Non-fatal error in "+operation, anyAttrs...)
}

// LogAndReturn logs an error and returns it without additional wrapping
// Used when the error is already properly formatted but needs logging
func LogAndReturn(logger *slog.Logger, operation string, err error, attrs ...slog.Attr) error {
	if logger == nil || err == nil {
		return err
	}

	logAttrs := []slog.Attr{
		slog.String("error", err.Error()),
	}
	logAttrs = append(logAttrs, attrs...)

	anyAttrs := make([]any, len(logAttrs))
	for i, attr := range logAttrs {
		anyAttrs[i] = attr
	}

	logger.Error(operation+" failed", anyAttrs...)
	return err
}

// ExecuteWithLogging wraps a function call with operation logging
// Automatically logs start/completion with timing and handles errors
func ExecuteWithLogging(logger *slog.Logger, operation string, fn func() error, attrs ...slog.Attr) error {
	if logger == nil {
		return fn()
	}

	start := time.Now()

	// Convert attrs to interface slice for start logging
	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	logger.Debug("Starting "+operation, anyAttrs...)

	err := fn()
	duration := time.Since(start)

	// Add duration to attributes for completion logging
	completionAttrs := append(attrs, slog.Duration("duration", duration))
	anyCompletionAttrs := make([]any, len(completionAttrs))
	for i, attr := range completionAttrs {
		anyCompletionAttrs[i] = attr
	}

	if err != nil {
		errorAttrs := append(completionAttrs, slog.String("error", err.Error()))
		anyErrorAttrs := make([]any, len(errorAttrs))
		for i, attr := range errorAttrs {
			anyErrorAttrs[i] = attr
		}
		logger.Error("Failed "+operation, anyErrorAttrs...)
		return fmt.Errorf("%s: %w", operation, err)
	}

	logger.Debug("Completed "+operation, anyCompletionAttrs...)
	return nil
}

// Common context helpers for frequently used attributes

// WeatherContext creates context attributes for weather operations
func WeatherContext(latitude, longitude float64, units string) []slog.Attr {
	return []slog.Attr{
		slog.Float64("latitude", latitude),
		slog.Float64("longitude", longitude),
		slog.String("units", units),
	}
}

// ConfigContext creates context attributes for configuration operations
func ConfigContext(configFile string) []slog.Attr {
	if configFile == "" {
		return nil
	}
	return []slog.Attr{slog.String("config_file", configFile)}
}

// FileContext creates context attributes for file operations
func FileContext(filePath string) []slog.Attr {
	if filePath == "" {
		return nil
	}
	return []slog.Attr{slog.String("file_path", filePath)}
}

// URLContext creates context attributes for URL/API operations
func URLContext(url string) []slog.Attr {
	if url == "" {
		return nil
	}
	return []slog.Attr{slog.String("url", url)}
}

// APIContext creates context attributes for API operations
func APIContext(provider, model string) []slog.Attr {
	attrs := make([]slog.Attr, 0, 2)
	if provider != "" {
		attrs = append(attrs, slog.String("api_provider", provider))
	}
	if model != "" {
		attrs = append(attrs, slog.String("model", model))
	}
	return attrs
}

// AudioContext creates context attributes for audio operations
func AudioContext(format string, sampleRate int, bitRate int) []slog.Attr {
	attrs := make([]slog.Attr, 0, 3)
	if format != "" {
		attrs = append(attrs, slog.String("audio_format", format))
	}
	if sampleRate > 0 {
		attrs = append(attrs, slog.Int("sample_rate", sampleRate))
	}
	if bitRate > 0 {
		attrs = append(attrs, slog.Int("bit_rate", bitRate))
	}
	return attrs
}
