package errorutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	neturl "net/url"
	"time"
)

// NetworkError represents a network-related error with additional context
type NetworkError struct {
	Operation   string        // The operation that failed (e.g., "API request", "DNS lookup")
	URL         string        // The URL that was being accessed
	StatusCode  int           // HTTP status code (if applicable)
	Timeout     time.Duration // Timeout duration (if applicable)
	Underlying  error         // The underlying error
	Retryable   bool          // Whether this error is retryable
}

func (e *NetworkError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s failed for %s: HTTP %d: %v", e.Operation, e.URL, e.StatusCode, e.Underlying)
	}
	return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.URL, e.Underlying)
}

func (e *NetworkError) Unwrap() error {
	return e.Underlying
}

// IsRetryable returns whether the error suggests retrying might be worthwhile
func (e *NetworkError) IsRetryable() bool {
	return e.Retryable
}

// NewNetworkError creates a new NetworkError with proper context
func NewNetworkError(operation, url string, err error) *NetworkError {
	netErr := &NetworkError{
		Operation:  operation,
		URL:        url,
		Underlying: err,
		Retryable:  isRetryableError(err),
	}

	// Extract HTTP status code if available
	if uErr, ok := err.(*neturl.Error); ok {
		if uErr.Err != nil {
			netErr.StatusCode = extractStatusCode(uErr.Err)
		}
	} else {
		netErr.StatusCode = extractStatusCode(err)
	}

	// Extract timeout information
	if isTimeoutError(err) {
		netErr.Timeout = extractTimeout(err)
	}

	return netErr
}

// LogNetworkError logs a network error with appropriate structured context
func LogNetworkError(logger *slog.Logger, netErr *NetworkError) *NetworkError {
	if logger == nil {
		return netErr
	}

	attrs := []slog.Attr{
		slog.String("operation", netErr.Operation),
		slog.String("url", netErr.URL),
		slog.String("error", netErr.Underlying.Error()),
		slog.Bool("retryable", netErr.Retryable),
	}

	if netErr.StatusCode > 0 {
		attrs = append(attrs, slog.Int("status_code", netErr.StatusCode))
	}

	if netErr.Timeout > 0 {
		attrs = append(attrs, slog.Duration("timeout", netErr.Timeout))
	}

	// Choose appropriate log level based on error type
	level := slog.LevelError
	if netErr.Retryable {
		level = slog.LevelWarn // Retryable errors are warnings
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	logger.Log(nil, level, "Network operation failed", anyAttrs...)
	return netErr
}

// isRetryableError determines if an error is likely to be resolved by retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network timeout errors are usually retryable
	if isTimeoutError(err) {
		return true
	}

	// DNS errors might be temporary
	if isDNSError(err) {
		return true
	}

	// Connection refused might be temporary (service starting up)
	if isConnectionRefusedError(err) {
		return true
	}

	// HTTP status codes that suggest retrying
	statusCode := extractStatusCode(err)
	switch statusCode {
	case http.StatusTooManyRequests,
		 http.StatusInternalServerError,
		 http.StatusBadGateway,
		 http.StatusServiceUnavailable,
		 http.StatusGatewayTimeout:
		return true
	}

	return false
}

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error timeout
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}

	// Check for url.Error with timeout
	if urlErr, ok := err.(*neturl.Error); ok {
		if netErr, ok := urlErr.Err.(net.Error); ok {
			return netErr.Timeout()
		}
	}

	// Check for context deadline exceeded
	return errors.Is(err, context.DeadlineExceeded)
}

// isDNSError checks if an error is a DNS resolution error
func isDNSError(err error) bool {
	if err == nil {
		return false
	}

	// Check for DNS error types
	if _, ok := err.(*net.DNSError); ok {
		return true
	}

	if urlErr, ok := err.(*neturl.Error); ok {
		if _, ok := urlErr.Err.(*net.DNSError); ok {
			return true
		}
	}

	return false
}

// isConnectionRefusedError checks if an error is a connection refused error
func isConnectionRefusedError(err error) bool {
	if err == nil {
		return false
	}

	// Check for connection refused in various error types
	errStr := err.Error()
	return contains(errStr, "connection refused") || 
		   contains(errStr, "connect: connection refused")
}

// extractStatusCode attempts to extract HTTP status code from various error types
func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}

	// Try to extract from HTTP response error
	if httpErr, ok := err.(*neturl.Error); ok {
		if httpErr.Err != nil {
			return extractStatusCode(httpErr.Err)
		}
	}

	// This would need to be expanded based on the HTTP client being used
	// For now, return 0 for unknown status codes
	return 0
}

// extractTimeout attempts to extract timeout duration from error
func extractTimeout(err error) time.Duration {
	// This is a simplified implementation
	// In practice, you might want to extract actual timeout values
	// from specific error types or context
	return 30 * time.Second // Default assumption
}

// contains is a helper function for case-insensitive string searching
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    len(s) > len(substr) && 
		    (s[:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr ||
		     anySubstring(s, substr)))
}

// anySubstring checks if substr exists anywhere in s
func anySubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RetryableHTTPError represents an HTTP error that may be worth retrying
type RetryableHTTPError struct {
	StatusCode int
	URL        string
	Method     string
	Body       string
	Headers    map[string]string
	Underlying error
}

func (e *RetryableHTTPError) Error() string {
	return fmt.Sprintf("%s %s failed with HTTP %d: %v", e.Method, e.URL, e.StatusCode, e.Underlying)
}

func (e *RetryableHTTPError) Unwrap() error {
	return e.Underlying
}

// NewRetryableHTTPError creates a new RetryableHTTPError
func NewRetryableHTTPError(method, url string, statusCode int, err error) *RetryableHTTPError {
	return &RetryableHTTPError{
		Method:     method,
		URL:        url,
		StatusCode: statusCode,
		Underlying: err,
	}
}