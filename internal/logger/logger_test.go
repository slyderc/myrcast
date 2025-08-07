package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoggerInitialization tests the initialization of the enhanced logger
func TestLoggerInitialization(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
	}{
		{
			name: "valid console-only config",
			config: Config{
				Enabled:       false,
				ConsoleOutput: true,
				Level:         "info",
			},
			wantError: false,
		},
		{
			name: "valid file logging config",
			config: Config{
				Enabled:         true,
				Directory:       t.TempDir(),
				FilenamePattern: "test-YYYYMMDD.log",
				Level:           "debug",
				ConsoleOutput:   true,
			},
			wantError: false,
		},
		{
			name: "invalid filename pattern",
			config: Config{
				Enabled:         true,
				Directory:       t.TempDir(),
				FilenamePattern: "test-MM/DD/YYYY.log", // Invalid: contains slashes
				Level:           "info",
			},
			wantError: true,
		},
		{
			name: "invalid log level defaults to info",
			config: Config{
				Enabled:       false,
				ConsoleOutput: true,
				Level:         "invalid-level",
			},
			wantError: false, // Should not error, just use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Initialize(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("Initialize() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestLogLevels tests that log levels are properly filtered
func TestLogLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "test.log",
		Level:           "warn",
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Log at different levels
	Debug("This debug message should not appear")
	Info("This info message should not appear")
	Warn("This warning should appear")
	Error("This error should appear")

	// Give time for async operations
	time.Sleep(100 * time.Millisecond)

	// Read log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check that only warn and error messages appear
	if strings.Contains(logContent, "debug message") {
		t.Error("Debug message appeared when log level was warn")
	}
	if strings.Contains(logContent, "info message") {
		t.Error("Info message appeared when log level was warn")
	}
	if !strings.Contains(logContent, "warning should appear") {
		t.Error("Warning message did not appear")
	}
	if !strings.Contains(logContent, "error should appear") {
		t.Error("Error message did not appear")
	}
}

// TestLogRotation tests file rotation based on size
func TestLogRotation(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "test-rotation.log",
		Level:           "info",
		MaxFiles:        3,
		MaxSizeMB:       1, // 1MB for testing
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	logger := Get()

	// Write enough data to trigger rotation (over 1MB)
	largeMessage := strings.Repeat("This is a test message for rotation. ", 100)
	iterations := 1024*1024/len(largeMessage) + 100 // Ensure we go over 1MB

	for i := 0; i < iterations; i++ {
		logger.Info(largeMessage)
	}

	// Give time for rotation
	time.Sleep(500 * time.Millisecond)

	// List all files to see what was created
	files, err := filepath.Glob(filepath.Join(tmpDir, "*.log"))
	if err != nil {
		t.Fatalf("Failed to glob log files: %v", err)
	}
	t.Logf("Log files found: %v", files)

	// Check that rotation occurred by checking file size
	logPath := filepath.Join(tmpDir, "test-rotation.log")
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	t.Logf("Log file size: %d bytes", info.Size())

	// File should be smaller than 1MB after rotation
	if info.Size() >= 1024*1024 {
		t.Errorf("Log file size %d bytes, expected less than 1MB after rotation", info.Size())
	}
}

// TestFilenamePatternGeneration tests the date pattern replacement
func TestFilenamePatternGeneration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		pattern  string
		contains []string
	}{
		{
			pattern: "test-YYYYMMDD.log",
			contains: []string{
				"test-",
				now.Format("20060102"),
				".log",
			},
		},
		{
			pattern: "app-YYYY-MM-DD.log",
			contains: []string{
				"app-",
				now.Format("2006"),
				now.Format("01"),
				now.Format("02"),
				".log",
			},
		},
		{
			pattern: "log-YYYY.MM.DD-HH.log",
			contains: []string{
				"log-",
				now.Format("2006"),
				now.Format("01"),
				now.Format("02"),
				now.Format("15"),
				".log",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := generateLogFilename(tt.pattern)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("generateLogFilename(%s) = %s, expected to contain %s",
						tt.pattern, result, expected)
				}
			}
		})
	}
}

// TestPlatformSpecificPaths tests platform-specific directory expansion
func TestPlatformSpecificPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		isAbs    bool
		contains string
	}{
		{
			name:     "relative logs directory",
			input:    "logs",
			isAbs:    false,
			contains: "logs",
		},
		{
			name:     "absolute path",
			input:    "/var/log/myrcast",
			isAbs:    true,
			contains: "/var/log/myrcast",
		},
		{
			name:     "empty defaults to logs",
			input:    "",
			isAbs:    false,
			contains: "logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandLogDirectory(tt.input)

			if tt.isAbs && !filepath.IsAbs(result) {
				t.Errorf("Expected absolute path, got %s", result)
			}

			if !strings.Contains(result, tt.contains) {
				t.Errorf("expandLogDirectory(%s) = %s, expected to contain %s",
					tt.input, result, tt.contains)
			}
		})
	}
}

// TestStructuredLogging tests the structured logging functions
func TestStructuredLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "structured.log")

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "structured.log",
		Level:           "debug",
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test API request/response logging
	headers := map[string]string{
		"User-Agent": "TestAgent/1.0",
	}
	LogAPIRequest("GET", "https://api.example.com/test", headers)
	LogAPIResponse("GET", "https://api.example.com/test", 200, "1.234s", 1024)

	// Test file operation logging
	LogFileOperation("write", "/tmp/test.txt", 2048)
	LogFileError("read", "/tmp/missing.txt", os.ErrNotExist)

	// Test operation start/complete
	complete := LogOperationStart("test-operation", map[string]any{
		"param1": "value1",
		"param2": 42,
	})
	time.Sleep(100 * time.Millisecond)
	complete(nil) // Success

	// Test structured error
	LogStructuredError(os.ErrPermission, map[string]any{
		"file": "/restricted/file",
		"mode": "write",
	})

	// Give time for writes
	time.Sleep(100 * time.Millisecond)

	// Read and verify log content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify structured fields appear in logs
	expectedStrings := []string{
		"api_request",
		"https://api.example.com/test",
		"TestAgent/1.0",
		"status_code=200",
		"file_operation",
		"operation_start",
		"operation_complete",
		"structured_error",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Log content missing expected string: %s", expected)
		}
	}
}

// TestLogWithFields tests custom field logging
func TestLogWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "fields.log")

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "fields.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Log with custom fields
	fields := map[string]any{
		"user_id":    12345,
		"request_id": "abc-123",
		"duration":   1.234,
		"success":    true,
	}

	LogWithFields(InfoLevel, "Custom message with fields", fields)

	// Give time for write
	time.Sleep(100 * time.Millisecond)

	// Read and verify
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check that all fields appear
	if !strings.Contains(logContent, "user_id=12345") {
		t.Error("user_id field not found in log")
	}
	if !strings.Contains(logContent, "request_id=abc-123") {
		t.Error("request_id field not found in log")
	}
	if !strings.Contains(logContent, "Custom message with fields") {
		t.Error("Log message not found")
	}
}

// TestExecutionSummary tests the execution summary logging
func TestExecutionSummary(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "summary.log")

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "summary.log",
		Level:           "info",
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	logger := Get()

	// Log execution summary
	startTime := time.Now().Add(-5 * time.Minute)
	results := []string{
		"Processed 100 files",
		"Generated 50 reports",
		"Completed successfully",
	}

	logger.LogExecutionSummary(startTime, "test-config.toml", "batch-process", results, 0)

	// Give time for write
	time.Sleep(100 * time.Millisecond)

	// Read and verify
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Check summary content
	expectedStrings := []string{
		"EXECUTION SUMMARY",
		"test-config.toml",
		"batch-process",
		"exit_code=0",
		"Processed 100 files",
		"Generated 50 reports",
		"Completed successfully",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(logContent, expected) {
			t.Errorf("Execution summary missing: %s", expected)
		}
	}
}

// TestParseLevel tests log level parsing
func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		hasError bool
	}{
		{"debug", DebugLevel, false},
		{"DEBUG", DebugLevel, false},
		{"info", InfoLevel, false},
		{"INFO", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"warning", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"ERROR", ErrorLevel, false},
		{"fatal", FatalLevel, false},
		{"invalid", InfoLevel, true},
		{"", InfoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("ParseLevel(%s) error = %v, hasError %v", tt.input, err, tt.hasError)
			}
			if level != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, expected %v", tt.input, level, tt.expected)
			}
		})
	}
}

// TestLoggerCleanup tests the file cleanup functionality
func TestLoggerCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	// Create old log files
	for i := 0; i < 5; i++ {
		oldDate := time.Now().AddDate(0, 0, -i-1)
		filename := strings.ReplaceAll("test-YYYYMMDD.log", "YYYYMMDD", oldDate.Format("20060102"))
		filepath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filepath, []byte("old log content"), 0644); err != nil {
			t.Fatalf("Failed to create old log file: %v", err)
		}
	}

	config := Config{
		Enabled:         true,
		Directory:       tmpDir,
		FilenamePattern: "test-YYYYMMDD.log",
		Level:           "info",
		MaxFiles:        3, // Keep only 3 files
		ConsoleOutput:   false,
	}

	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	logger := Get()
	logger.Info("New log entry")

	// Call cleanup synchronously since we're in the same package
	logger.cleanOldFiles()

	// Give a small buffer for file operations to complete
	time.Sleep(100 * time.Millisecond)

	// Count remaining files
	files, err := filepath.Glob(filepath.Join(tmpDir, "test-*.log"))
	if err != nil {
		t.Fatalf("Failed to glob log files: %v", err)
	}

	if len(files) > 3 {
		t.Errorf("Expected max 3 log files, found %d", len(files))
	}
}
