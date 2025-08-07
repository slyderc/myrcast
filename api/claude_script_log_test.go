package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendScriptToLog(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a mock Claude client
	client := &ClaudeClient{}

	// Create initial log content with the structure that logPromptToFile would create
	initialContent := `=== CLAUDE WEATHER REPORT GENERATION ===
Timestamp: 2024-01-15 12:00:00 PST
Location: San Francisco, CA

=== WEATHER DATA (System Context) ===
Current temperature: 65°F
High: 70°F, Low: 55°F

=== USER PROMPT TEMPLATE ===
Generate a weather report...

=== CLAUDE API PARAMETERS ===
Model: claude-3-5-sonnet-20241022
Max Tokens: 1000
Temperature: 0.70

=== END LOG ENTRY ===
`

	// Write initial content to results.log
	logFile := filepath.Join(tempDir, "results.log")
	if err := os.WriteFile(logFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	// Create test request
	request := WeatherReportRequest{
		OutputPath: tempDir,
	}

	// Test script to append
	testScript := "Good morning! Today's weather in San Francisco shows partly cloudy skies with a high of 70 degrees and a low of 55 degrees. Perfect conditions for your morning commute!"

	// Test the appendScriptToLog function
	err := client.appendScriptToLog(request, testScript)
	if err != nil {
		t.Fatalf("appendScriptToLog failed: %v", err)
	}

	// Read the modified log file
	modifiedContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read modified log file: %v", err)
	}

	modifiedStr := string(modifiedContent)

	// Verify the script section was added
	if !strings.Contains(modifiedStr, "=== CLAUDE WEATHER SCRIPT ===") {
		t.Error("Script header not found in log file")
	}

	// Verify the script content was added
	if !strings.Contains(modifiedStr, testScript) {
		t.Error("Script content not found in log file")
	}

	// Verify the end marker is still present
	if !strings.Contains(modifiedStr, "=== END LOG ENTRY ===") {
		t.Error("End log entry marker not found in modified log file")
	}

	// Verify the script section appears before the end marker
	scriptIndex := strings.Index(modifiedStr, "=== CLAUDE WEATHER SCRIPT ===")
	endIndex := strings.Index(modifiedStr, "=== END LOG ENTRY ===")

	if scriptIndex == -1 || endIndex == -1 {
		t.Fatal("Required markers not found")
	}

	if scriptIndex >= endIndex {
		t.Error("Script section should appear before the end marker")
	}

	// Verify original content is preserved
	if !strings.Contains(modifiedStr, "Timestamp: 2024-01-15 12:00:00 PST") {
		t.Error("Original timestamp not preserved")
	}

	if !strings.Contains(modifiedStr, "Location: San Francisco, CA") {
		t.Error("Original location not preserved")
	}
}

func TestAppendScriptToLog_MissingEndMarker(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a mock Claude client
	client := &ClaudeClient{}

	// Create log content without the end marker
	initialContent := `=== CLAUDE WEATHER REPORT GENERATION ===
Timestamp: 2024-01-15 12:00:00 PST
Location: San Francisco, CA
`

	// Write initial content to results.log
	logFile := filepath.Join(tempDir, "results.log")
	if err := os.WriteFile(logFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	// Create test request
	request := WeatherReportRequest{
		OutputPath: tempDir,
	}

	// Test script to append
	testScript := "Test script content"

	// Test the appendScriptToLog function - should return error
	err := client.appendScriptToLog(request, testScript)
	if err == nil {
		t.Error("Expected error when end marker is missing, but got nil")
	}

	if !strings.Contains(err.Error(), "end log entry marker not found") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestAppendScriptToLog_MissingLogFile(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()

	// Create a mock Claude client
	client := &ClaudeClient{}

	// Create test request pointing to non-existent log file
	request := WeatherReportRequest{
		OutputPath: tempDir,
	}

	// Test script to append
	testScript := "Test script content"

	// Test the appendScriptToLog function - should return error
	err := client.appendScriptToLog(request, testScript)
	if err == nil {
		t.Error("Expected error when log file is missing, but got nil")
	}

	if !strings.Contains(err.Error(), "failed to read existing results.log") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}
