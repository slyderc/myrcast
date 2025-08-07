package logger

import (
	"runtime"
	"strings"
	"testing"
)

// TestValidateFilenamePattern tests filename pattern validation
func TestValidateFilenamePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		shouldError bool
		errorType   string
	}{
		// Valid patterns
		{
			name:        "simple daily pattern",
			pattern:     "app-YYYYMMDD.log",
			shouldError: false,
		},
		{
			name:        "pattern with time",
			pattern:     "app-YYYYMMDD-HHMMSS.log",
			shouldError: false,
		},
		{
			name:        "pattern with dashes",
			pattern:     "app-YYYY-MM-DD.log",
			shouldError: false,
		},
		{
			name:        "pattern with dots",
			pattern:     "app.YYYY.MM.DD.log",
			shouldError: false,
		},
		{
			name:        "pattern with underscores",
			pattern:     "app_YYYY_MM_DD.log",
			shouldError: false,
		},
		{
			name:        "empty pattern uses default",
			pattern:     "",
			shouldError: false,
		},

		// Invalid patterns - universal
		{
			name:        "pattern with forward slashes",
			pattern:     "app-MM/DD/YYYY.log",
			shouldError: true,
			errorType:   "'/'",
		},
		{
			name:        "pattern with backslashes",
			pattern:     "app\\YYYY\\MM.log",
			shouldError: true,
			errorType:   "'\\'",
		},

		// Invalid patterns - Windows specific
		{
			name:        "pattern with colon",
			pattern:     "app-HH:MM:SS.log",
			shouldError: runtime.GOOS == "windows",
			errorType:   "colon",
		},
		{
			name:        "pattern with pipe",
			pattern:     "app-YYYY|MM|DD.log",
			shouldError: runtime.GOOS == "windows",
			errorType:   "pipe",
		},
		{
			name:        "pattern with asterisk",
			pattern:     "app-*-YYYYMMDD.log",
			shouldError: runtime.GOOS == "windows",
			errorType:   "asterisk",
		},
		{
			name:        "pattern with question mark",
			pattern:     "app-?-YYYYMMDD.log",
			shouldError: runtime.GOOS == "windows",
			errorType:   "question mark",
		},
		{
			name:        "pattern with angle brackets",
			pattern:     "app-<YYYY>.log",
			shouldError: runtime.GOOS == "windows",
			errorType:   "angle brackets",
		},
		{
			name:        "pattern with quotes",
			pattern:     `app-"YYYY".log`,
			shouldError: runtime.GOOS == "windows",
			errorType:   "quotes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilenamePattern(tt.pattern)
			if (err != nil) != tt.shouldError {
				t.Errorf("ValidateFilenamePattern(%s) error = %v, shouldError %v",
					tt.pattern, err, tt.shouldError)
			}

			if err != nil && tt.shouldError {
				// Verify it's a FilenameValidationError
				if _, ok := err.(*FilenameValidationError); !ok {
					t.Errorf("Expected FilenameValidationError, got %T", err)
				}

				// Check that error message contains expected context
				errMsg := err.Error()
				if tt.errorType != "" && !containsAny(errMsg, tt.errorType) {
					t.Errorf("Error message doesn't mention %s: %s", tt.errorType, errMsg)
				}
			}
		})
	}
}

// TestFilenameValidationError tests the custom error type
func TestFilenameValidationError(t *testing.T) {
	err := &FilenameValidationError{
		Pattern:      "test-MM/DD/YYYY.log",
		InvalidChars: []rune{'/', '/'},
		Platform:     "Windows",
		Suggestion:   "test-MM-DD-YYYY.log",
	}

	errMsg := err.Error()

	// Check error message components
	expectedParts := []string{
		"invalid filename pattern",
		"test-MM/DD/YYYY.log",
		"'/'",
		"Windows",
		"test-MM-DD-YYYY.log",
	}

	for _, expected := range expectedParts {
		if !containsAny(errMsg, expected) {
			t.Errorf("Error message missing expected part: %s\nFull message: %s",
				expected, errMsg)
		}
	}
}

// TestGetSafeFilenamePatterns tests the safe pattern recommendations
func TestGetSafeFilenamePatterns(t *testing.T) {
	patterns := GetSafeFilenamePatterns()

	if len(patterns) == 0 {
		t.Error("GetSafeFilenamePatterns returned no patterns")
	}

	// All safe patterns should validate successfully
	for _, pattern := range patterns {
		if err := ValidateFilenamePattern(pattern); err != nil {
			t.Errorf("Safe pattern %s failed validation: %v", pattern, err)
		}
	}
}

// TestGetUnsafeFilenamePatterns tests the unsafe pattern examples
func TestGetUnsafeFilenamePatterns(t *testing.T) {
	unsafePatterns := GetUnsafeFilenamePatterns()

	if len(unsafePatterns) == 0 {
		t.Error("GetUnsafeFilenamePatterns returned no patterns")
	}

	// Verify each unsafe pattern has a reason
	for pattern, reason := range unsafePatterns {
		if reason == "" {
			t.Errorf("Unsafe pattern %s has no reason", pattern)
		}

		// Most unsafe patterns should fail validation
		// (some may be platform-specific)
		err := ValidateFilenamePattern(pattern)
		if err == nil && containsAny(pattern, "/", "\\") {
			t.Errorf("Unsafe pattern %s with path separators passed validation", pattern)
		}
	}
}

// TestIsAbsolutePath tests absolute path detection
func TestIsAbsolutePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
		platform string
	}{
		// Unix paths
		{"/absolute/path", true, "unix"},
		{"relative/path", false, "unix"},
		{"./relative", false, "unix"},

		// Windows paths
		{`C:\Windows\Path`, true, "windows"},
		{`D:\`, true, "windows"},
		{`\\server\share`, true, "windows"},
		{`relative\path`, false, "windows"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			// Skip platform-specific tests on wrong platform
			if tt.platform == "windows" && runtime.GOOS != "windows" {
				return
			}
			if tt.platform == "unix" && runtime.GOOS == "windows" {
				return
			}

			result := isAbsolutePath(tt.path)
			if result != tt.expected {
				t.Errorf("isAbsolutePath(%s) = %v, expected %v",
					tt.path, result, tt.expected)
			}
		})
	}
}

// TestExtractFilename tests filename extraction from paths
func TestExtractFilename(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		// Unix paths
		{"/var/log/app.log", "app.log"},
		{"logs/app.log", "app.log"},
		{"app.log", "app.log"},

		// Windows paths
		{`C:\Logs\app.log`, "app.log"},
		{`logs\app.log`, "app.log"},

		// Mixed separators
		{"logs/subfolder\\app.log", "app.log"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := extractFilename(tt.pattern)
			if result != tt.expected {
				t.Errorf("extractFilename(%s) = %s, expected %s",
					tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestExtractDirectory tests directory extraction from paths
func TestExtractDirectory(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		// Unix paths
		{"/var/log/app.log", "/var/log"},
		{"logs/app.log", "logs"},
		{"app.log", ""},

		// Windows paths
		{`C:\Logs\app.log`, `C:\Logs`},
		{`logs\app.log`, `logs`},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := extractDirectory(tt.pattern)
			if result != tt.expected {
				t.Errorf("extractDirectory(%s) = %s, expected %s",
					tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestFindInvalidCharsInFilename tests character validation
func TestFindInvalidCharsInFilename(t *testing.T) {
	tests := []struct {
		filename      string
		expectInvalid bool
		platform      string
	}{
		// Valid filenames
		{"normal-file.log", false, "all"},
		{"file_with_underscores.log", false, "all"},
		{"file.with.dots.log", false, "all"},

		// Invalid on all platforms
		{"file\x00null.log", true, "all"},

		// Invalid on Windows only
		{"file:with:colons.log", runtime.GOOS == "windows", "windows"},
		{"file|with|pipes.log", runtime.GOOS == "windows", "windows"},
		{"file*with*stars.log", runtime.GOOS == "windows", "windows"},
		{"file?with?questions.log", runtime.GOOS == "windows", "windows"},
		{"file<with>brackets.log", runtime.GOOS == "windows", "windows"},
		{`file"with"quotes.log`, runtime.GOOS == "windows", "windows"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			invalidChars := findInvalidCharsInFilename(tt.filename)
			hasInvalid := len(invalidChars) > 0

			if hasInvalid != tt.expectInvalid {
				t.Errorf("findInvalidCharsInFilename(%s) found %v chars, expectInvalid=%v",
					tt.filename, invalidChars, tt.expectInvalid)
			}
		})
	}
}

// TestGetSuggestionForFilename tests filename suggestions
func TestGetSuggestionForFilename(t *testing.T) {
	tests := []struct {
		fullPattern  string
		filename     string
		invalidChars []rune
		expectIn     string
	}{
		{
			fullPattern:  "/var/log/app-MM:DD:YYYY.log",
			filename:     "app-MM:DD:YYYY.log",
			invalidChars: []rune{':'},
			expectIn:     "MM-DD-YYYY",
		},
		{
			fullPattern:  "/var/log/app-HH:MM:SS.log",
			filename:     "app-HH:MM:SS.log",
			invalidChars: []rune{':'},
			expectIn:     "HH-MM-SS",
		},
		{
			fullPattern:  "/var/log/app-HH:MM.log",
			filename:     "app-HH:MM.log",
			invalidChars: []rune{':'},
			expectIn:     "app-HH-MM.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fullPattern, func(t *testing.T) {
			suggestion := getSuggestionForFilename(tt.fullPattern, tt.filename, tt.invalidChars)

			if !containsAny(suggestion, tt.expectIn) {
				t.Errorf("getSuggestionForFilename(%s) = %s, expected to contain %s",
					tt.fullPattern, suggestion, tt.expectIn)
			}

			// The suggestion should not contain any invalid chars
			for _, char := range tt.invalidChars {
				if containsRune(suggestion, char) {
					t.Errorf("Suggestion %s still contains invalid char %c",
						suggestion, char)
				}
			}
		})
	}
}

// Helper functions
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) && strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func containsRune(s string, r rune) bool {
	for _, char := range s {
		if char == r {
			return true
		}
	}
	return false
}
