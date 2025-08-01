package errorutil

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestLogAndWrap tests the LogAndWrap functionality
func TestLogAndWrap(t *testing.T) {
	// Create a test logger with custom handler to capture output at DEBUG level
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug}))

	originalErr := errors.New("original error")
	operation := "test operation"

	// Test with additional attributes
	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	wrappedErr := LogAndWrap(logger, operation, originalErr, attrs...)

	// Check error wrapping
	if wrappedErr == nil {
		t.Fatal("LogAndWrap returned nil error")
	}

	if !strings.Contains(wrappedErr.Error(), operation) {
		t.Errorf("Wrapped error doesn't contain operation: %v", wrappedErr)
	}

	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Wrapped error doesn't unwrap to original error")
	}

	// Check log output
	logStr := logOutput.String()
	if !strings.Contains(logStr, "test operation failed") {
		t.Error("Log doesn't contain expected message")
	}
	if !strings.Contains(logStr, "original error") {
		t.Error("Log doesn't contain original error")
	}
	if !strings.Contains(logStr, "key1=value1") {
		t.Error("Log doesn't contain custom attribute")
	}
}

// TestLogWarning tests the LogWarning functionality
func TestLogWarning(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	err := errors.New("warning error")
	operation := "risky operation"

	LogWarning(logger, operation, err, slog.String("context", "test"))

	logStr := logOutput.String()
	if !strings.Contains(logStr, "Non-fatal error") {
		t.Error("Log doesn't contain warning prefix")
	}
	if !strings.Contains(logStr, operation) {
		t.Error("Log doesn't contain operation")
	}
	if !strings.Contains(logStr, "level=WARN") {
		t.Error("Log is not at WARN level")
	}
}

// TestLogAndReturn tests the LogAndReturn functionality
func TestLogAndReturn(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	originalErr := errors.New("return error")
	operation := "return operation"

	returnedErr := LogAndReturn(logger, operation, originalErr)

	// Should return the same error
	if returnedErr != originalErr {
		t.Error("LogAndReturn modified the error")
	}

	// Check log output
	logStr := logOutput.String()
	if !strings.Contains(logStr, operation+" failed") {
		t.Error("Log doesn't contain expected message")
	}
}

// TestExecuteWithLogging tests the ExecuteWithLogging wrapper
func TestExecuteWithLogging(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("successful execution", func(t *testing.T) {
		logOutput.Reset()
		executed := false
		
		err := ExecuteWithLogging(logger, "successful operation", func() error {
			executed = true
			time.Sleep(50 * time.Millisecond)
			return nil
		}, slog.String("test", "value"))

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !executed {
			t.Error("Function was not executed")
		}

		logStr := logOutput.String()
		if !strings.Contains(logStr, "Starting successful operation") {
			t.Error("Missing start log")
		}
		if !strings.Contains(logStr, "Completed successful operation") {
			t.Error("Missing completion log")
		}
		if !strings.Contains(logStr, "duration=") {
			t.Error("Missing duration in log")
		}
	})

	t.Run("failed execution", func(t *testing.T) {
		logOutput.Reset()
		testErr := errors.New("execution failed")
		
		err := ExecuteWithLogging(logger, "failing operation", func() error {
			return testErr
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, testErr) {
			t.Error("Error doesn't unwrap to original")
		}

		logStr := logOutput.String()
		if !strings.Contains(logStr, "Failed failing operation") {
			t.Error("Missing failure log")
		}
		if !strings.Contains(logStr, "execution failed") {
			t.Error("Missing error message in log")
		}
	})
}

// TestContextHelpers tests the context helper functions
func TestContextHelpers(t *testing.T) {
	t.Run("WeatherContext", func(t *testing.T) {
		attrs := WeatherContext(37.7749, -122.4194, "imperial")
		if len(attrs) != 3 {
			t.Errorf("Expected 3 attributes, got %d", len(attrs))
		}
		
		// Check attribute values
		found := make(map[string]bool)
		for _, attr := range attrs {
			found[attr.Key] = true
		}
		
		if !found["latitude"] || !found["longitude"] || !found["units"] {
			t.Error("Missing expected weather context attributes")
		}
	})

	t.Run("ConfigContext", func(t *testing.T) {
		attrs := ConfigContext("test.toml")
		if len(attrs) != 1 {
			t.Errorf("Expected 1 attribute, got %d", len(attrs))
		}
		if attrs[0].Key != "config_file" {
			t.Error("Wrong attribute key for config context")
		}
		
		// Test empty config
		emptyAttrs := ConfigContext("")
		if len(emptyAttrs) != 0 {
			t.Error("Empty config should return no attributes")
		}
	})

	t.Run("FileContext", func(t *testing.T) {
		attrs := FileContext("/tmp/test.txt")
		if len(attrs) != 1 {
			t.Errorf("Expected 1 attribute, got %d", len(attrs))
		}
		if attrs[0].Key != "file_path" {
			t.Error("Wrong attribute key for file context")
		}
	})

	t.Run("URLContext", func(t *testing.T) {
		attrs := URLContext("https://api.example.com")
		if len(attrs) != 1 {
			t.Errorf("Expected 1 attribute, got %d", len(attrs))
		}
		if attrs[0].Key != "url" {
			t.Error("Wrong attribute key for URL context")
		}
	})

	t.Run("APIContext", func(t *testing.T) {
		attrs := APIContext("openweather", "v2.5")
		if len(attrs) != 2 {
			t.Errorf("Expected 2 attributes, got %d", len(attrs))
		}
		
		// Test partial context
		partialAttrs := APIContext("", "model-only")
		if len(partialAttrs) != 1 {
			t.Error("Should return only non-empty attributes")
		}
	})

	t.Run("AudioContext", func(t *testing.T) {
		attrs := AudioContext("mp3", 44100, 128)
		if len(attrs) != 3 {
			t.Errorf("Expected 3 attributes, got %d", len(attrs))
		}
		
		// Test with zero values
		partialAttrs := AudioContext("wav", 0, 0)
		if len(partialAttrs) != 1 {
			t.Error("Should only include non-zero values")
		}
	})
}

// TestNilLoggerHandling tests that functions handle nil loggers gracefully
func TestNilLoggerHandling(t *testing.T) {
	err := errors.New("test error")

	// LogAndWrap with nil logger should just wrap the error
	wrappedErr := LogAndWrap(nil, "operation", err)
	if wrappedErr != err {
		t.Error("LogAndWrap with nil logger should return original error")
	}

	// LogWarning with nil logger should not panic
	LogWarning(nil, "operation", err)

	// LogAndReturn with nil logger should return original error
	returnedErr := LogAndReturn(nil, "operation", err)
	if returnedErr != err {
		t.Error("LogAndReturn with nil logger should return original error")
	}

	// ExecuteWithLogging with nil logger should just execute
	executed := false
	ExecuteWithLogging(nil, "operation", func() error {
		executed = true
		return nil
	})
	if !executed {
		t.Error("ExecuteWithLogging with nil logger should still execute function")
	}
}

// TestLogAndWrapWithNilError tests handling of nil errors
func TestLogAndWrapWithNilError(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	// Should return nil for nil error
	result := LogAndWrap(logger, "operation", nil)
	if result != nil {
		t.Error("LogAndWrap should return nil for nil error")
	}

	// Should not log anything
	if logOutput.Len() > 0 {
		t.Error("LogAndWrap should not log for nil error")
	}
}

// BenchmarkLogAndWrap benchmarks the LogAndWrap function
func BenchmarkLogAndWrap(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := errors.New("benchmark error")
	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LogAndWrap(logger, "benchmark operation", err, attrs...)
	}
}

// BenchmarkExecuteWithLogging benchmarks the ExecuteWithLogging wrapper
func BenchmarkExecuteWithLogging(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteWithLogging(logger, "benchmark", func() error {
			// Simulate some work
			time.Sleep(1 * time.Microsecond)
			return nil
		})
	}
}