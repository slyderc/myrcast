package errorutil

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field       string      // The field that failed validation
	Value       interface{} // The value that was being validated
	Rule        string      // The validation rule that failed
	Message     string      // Human-readable error message
	Suggestions []string    // Suggested corrections
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed for field '%s' with rule '%s'", e.Field, e.Rule)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("validation failed with %d errors: %s", len(e.Errors), e.Errors[0].Error())
}

// Add adds a validation error to the collection
func (e *ValidationErrors) Add(field, rule, message string, value interface{}, suggestions ...string) {
	e.Errors = append(e.Errors, ValidationError{
		Field:       field,
		Value:       value,
		Rule:        rule,
		Message:     message,
		Suggestions: suggestions,
	})
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// LogValidationErrors logs validation errors with structured context
func LogValidationErrors(logger *slog.Logger, valErr *ValidationErrors) *ValidationErrors {
	if logger == nil || !valErr.HasErrors() {
		return valErr
	}

	for _, err := range valErr.Errors {
		attrs := []slog.Attr{
			slog.String("field", err.Field),
			slog.String("rule", err.Rule),
			slog.String("message", err.Message),
			slog.Any("value", err.Value),
		}

		if len(err.Suggestions) > 0 {
			attrs = append(attrs, slog.Any("suggestions", err.Suggestions))
		}

		anyAttrs := make([]any, len(attrs))
		for i, attr := range attrs {
			anyAttrs[i] = attr
		}

		logger.Warn("Validation error", anyAttrs...)
	}

	return valErr
}

// ValidateRequired checks if a field has a non-empty value
func ValidateRequired(field string, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "required",
			Message: "field is required and cannot be empty",
		}
	}
	return nil
}

// ValidateRange checks if a numeric value is within a specified range
func ValidateRange(field string, value float64, min, max float64) *ValidationError {
	if value < min || value > max {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "range",
			Message: fmt.Sprintf("value must be between %.2f and %.2f, got %.2f", min, max, value),
			Suggestions: []string{
				fmt.Sprintf("Try a value between %.2f and %.2f", min, max),
			},
		}
	}
	return nil
}

// ValidateIntRange checks if an integer value is within a specified range
func ValidateIntRange(field string, value, min, max int) *ValidationError {
	if value < min || value > max {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "int_range",
			Message: fmt.Sprintf("value must be between %d and %d, got %d", min, max, value),
			Suggestions: []string{
				fmt.Sprintf("Try a value between %d and %d", min, max),
			},
		}
	}
	return nil
}

// ValidateEnum checks if a value is one of the allowed enum values
func ValidateEnum(field string, value string, allowedValues []string) *ValidationError {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, allowed := range allowedValues {
		if strings.ToLower(allowed) == value {
			return nil
		}
	}

	return &ValidationError{
		Field:       field,
		Value:       value,
		Rule:        "enum",
		Message:     fmt.Sprintf("value must be one of: %s, got '%s'", strings.Join(allowedValues, ", "), value),
		Suggestions: allowedValues,
	}
}

// ValidateURL checks if a string is a valid URL
func ValidateURL(field string, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return ValidateRequired(field, value)
	}

	if _, err := url.Parse(value); err != nil {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "url",
			Message: fmt.Sprintf("invalid URL format: %v", err),
			Suggestions: []string{
				"Ensure URL starts with http:// or https://",
				"Check for typos in the URL",
			},
		}
	}

	return nil
}

// ValidateEmail checks if a string is a valid email address
func ValidateEmail(field string, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return ValidateRequired(field, value)
	}

	// Simple email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "email",
			Message: "invalid email address format",
			Suggestions: []string{
				"Ensure email has format: user@domain.com",
				"Check for typos in email address",
			},
		}
	}

	return nil
}

// ValidateFilePath checks if a file path is valid and accessible
func ValidateFilePath(field string, path string, mustExist bool) *ValidationError {
	if strings.TrimSpace(path) == "" {
		return ValidateRequired(field, path)
	}

	// Check for invalid characters in path
	invalidChars := []string{"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return &ValidationError{
				Field:   field,
				Value:   path,
				Rule:    "filepath",
				Message: "path contains invalid characters",
				Suggestions: []string{
					"Remove control characters from path",
					"Use only standard ASCII characters in paths",
				},
			}
		}
	}

	// Check if file must exist
	if mustExist {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return &ValidationError{
				Field:   field,
				Value:   path,
				Rule:    "file_exists",
				Message: "file or directory does not exist",
				Suggestions: []string{
					"Check that the path is correct",
					"Ensure the file has been created",
					"Verify you have permission to access the path",
				},
			}
		}
	}

	return nil
}

// ValidateCoordinate checks if a coordinate is within valid range
func ValidateCoordinate(field string, value float64, isLatitude bool) *ValidationError {
	var min, max float64
	var coordType string

	if isLatitude {
		min, max = -90.0, 90.0
		coordType = "latitude"
	} else {
		min, max = -180.0, 180.0
		coordType = "longitude"
	}

	if value < min || value > max {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "coordinate",
			Message: fmt.Sprintf("%s must be between %.1f and %.1f, got %.6f", coordType, min, max, value),
			Suggestions: []string{
				fmt.Sprintf("Valid %s range is %.1f to %.1f", coordType, min, max),
				"Check coordinate format (decimal degrees)",
			},
		}
	}

	return nil
}

// ValidateAPIKey checks if an API key has a reasonable format
func ValidateAPIKey(field string, value string, minLength int) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   field,
			Value:   "[REDACTED]",
			Rule:    "required",
			Message: "API key is required",
			Suggestions: []string{
				"Obtain API key from the service provider",
				"Check configuration file for missing key",
			},
		}
	}

	if len(value) < minLength {
		return &ValidationError{
			Field:   field,
			Value:   "[REDACTED]",
			Rule:    "min_length",
			Message: fmt.Sprintf("API key too short, expected at least %d characters", minLength),
			Suggestions: []string{
				"Verify complete API key was copied",
				"Check for truncated key in configuration",
			},
		}
	}

	// Check for placeholder values
	placeholders := []string{
		"your-api-key-here",
		"your-key-here",
		"replace-with-your-key",
		"xxx",
		"yyy",
		"zzz",
		"example",
		"test",
	}

	lowerValue := strings.ToLower(value)
	for _, placeholder := range placeholders {
		if strings.Contains(lowerValue, placeholder) {
			return &ValidationError{
				Field:   field,
				Value:   "[REDACTED]",
				Rule:    "placeholder",
				Message: "API key appears to be a placeholder value",
				Suggestions: []string{
					"Replace placeholder with actual API key",
					"Obtain real API key from service provider",
				},
			}
		}
	}

	return nil
}

// ValidatePositiveNumber checks if a string represents a positive number
func ValidatePositiveNumber(field string, value string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return ValidateRequired(field, value)
	}

	num, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "number",
			Message: "value must be a valid number",
			Suggestions: []string{
				"Use numeric format (e.g., 1.5, 42)",
				"Remove any non-numeric characters",
			},
		}
	}

	if num <= 0 {
		return &ValidationError{
			Field:   field,
			Value:   value,
			Rule:    "positive",
			Message: "value must be positive (greater than 0)",
			Suggestions: []string{
				"Use a positive number greater than 0",
			},
		}
	}

	return nil
}
