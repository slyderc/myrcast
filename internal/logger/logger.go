package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents logging severity using slog levels
type Level slog.Level

const (
	DebugLevel Level = Level(slog.LevelDebug)
	InfoLevel  Level = Level(slog.LevelInfo)
	WarnLevel  Level = Level(slog.LevelWarn)
	ErrorLevel Level = Level(slog.LevelError)
	FatalLevel Level = Level(slog.LevelError + 4) // Custom level above ERROR
)

// Config represents logging configuration compatible with main config package
type Config struct {
	Enabled         bool   `toml:"enabled"`
	Directory       string `toml:"directory"`
	FilenamePattern string `toml:"filename_pattern"`
	Level           string `toml:"level"`
	MaxFiles        int    `toml:"max_files"`
	MaxSizeMB       int    `toml:"max_size_mb"`
	ConsoleOutput   bool   `toml:"console_output"`
}

// EnhancedLogger wraps slog.Logger with rotation and file management capabilities
type EnhancedLogger struct {
	*slog.Logger
	config      Config
	file        *os.File
	fileName    string
	fileSize    int64
	mu          sync.Mutex
	multiWriter io.Writer
}

var (
	// Global enhanced logger instance
	globalLogger *EnhancedLogger
	globalMu     sync.Mutex
)

// Initialize creates and configures the global logger instance with the given configuration
func Initialize(config Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()
	
	var err error
	globalLogger, err = NewEnhancedLogger(config)
	return err
}

// Get returns the global logger instance, creating a fallback console logger if not initialized
func Get() *EnhancedLogger {
	if globalLogger == nil {
		// Fallback to console-only logger if not initialized
		consoleLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		globalLogger = &EnhancedLogger{Logger: consoleLogger}
	}
	return globalLogger
}

// NewEnhancedLogger creates a new enhanced logger with the given configuration
func NewEnhancedLogger(config Config) (*EnhancedLogger, error) {
	// Validate filename pattern for cross-platform compatibility if file logging is enabled
	if config.Enabled && config.FilenamePattern != "" {
		if err := ValidateFilenamePattern(config.FilenamePattern); err != nil {
			return nil, fmt.Errorf("invalid filename pattern: %w", err)
		}
	}

	logger := &EnhancedLogger{
		config: config,
	}

	// Parse log level
	level := parseLogLevel(config.Level)

	// Set up writers based on configuration
	writers := []io.Writer{}

	if config.ConsoleOutput {
		writers = append(writers, os.Stdout)
	}

	if config.Enabled {
		// Create log directory
		logDir := expandLogDirectory(config.Directory)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open log file
		logFile, err := logger.openLogFile()
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.file = logFile
		writers = append(writers, logFile)
	}

	// Create multi-writer
	if len(writers) == 0 {
		// Fallback to stdout if no writers configured
		writers = append(writers, os.Stdout)
	}
	logger.multiWriter = io.MultiWriter(writers...)

	// Create slog handler with custom formatting - use logger as writer for rotation
	handler := slog.NewTextHandler(logger, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Custom time format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02T15:04:05.000-07:00"))
			}
			// Shorten source paths for readability
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", filepath.Base(source.File), source.Line))
				}
			}
			return a
		},
	})

	logger.Logger = slog.New(handler)
	
	// Log initialization
	logger.Info("Enhanced logger initialized",
		slog.String("log_file", logger.fileName),
		slog.String("level", config.Level),
		slog.Bool("console", config.ConsoleOutput))

	return logger, nil
}

// openLogFile creates or opens the current log file
func (l *EnhancedLogger) openLogFile() (*os.File, error) {
	return l.openLogFileUnsafe()
}

// openLogFileUnsafe creates or opens the current log file (caller must hold mutex)
func (l *EnhancedLogger) openLogFileUnsafe() (*os.File, error) {
	logDir := expandLogDirectory(l.config.Directory)
	
	// Generate filename from pattern
	fileName := generateLogFilename(l.config.FilenamePattern)
	filePath := filepath.Join(logDir, fileName)
	
	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	
	// Get file info for size tracking
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	
	l.fileName = filePath
	l.fileSize = info.Size()
	
	return file, nil
}

// expandLogDirectory expands the log directory path with platform-specific defaults
func expandLogDirectory(dir string) string {
	if dir == "" {
		dir = "logs"
	}
	
	// Handle absolute paths
	if filepath.IsAbs(dir) {
		return dir
	}
	
	// Handle relative paths
	if dir == "logs" || strings.HasPrefix(dir, "./") {
		// Use working directory
		return dir
	}
	
	// Platform-specific default directories
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "Myrcast", "logs")
		}
	case "darwin", "linux":
		home := os.Getenv("HOME")
		if home != "" {
			return filepath.Join(home, ".myrcast", "logs")
		}
	}
	
	// Fallback to working directory
	return "logs"
}

// generateLogFilename creates a filename from the pattern using date formatting
func generateLogFilename(pattern string) string {
	if pattern == "" {
		pattern = "myrcast-YYYYMMDD.log"
	}
	
	now := time.Now()
	result := pattern
	
	// Replace date tokens
	result = strings.ReplaceAll(result, "YYYY", fmt.Sprintf("%04d", now.Year()))
	result = strings.ReplaceAll(result, "YY", fmt.Sprintf("%02d", now.Year()%100))
	result = strings.ReplaceAll(result, "MM", fmt.Sprintf("%02d", now.Month()))
	result = strings.ReplaceAll(result, "M", fmt.Sprintf("%d", now.Month()))
	result = strings.ReplaceAll(result, "DD", fmt.Sprintf("%02d", now.Day()))
	result = strings.ReplaceAll(result, "D", fmt.Sprintf("%d", now.Day()))
	result = strings.ReplaceAll(result, "HH", fmt.Sprintf("%02d", now.Hour()))
	result = strings.ReplaceAll(result, "H", fmt.Sprintf("%d", now.Hour()))
	
	return result
}

// parseLogLevel converts string level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// checkRotation checks if log rotation is needed
func (l *EnhancedLogger) checkRotation() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.checkRotationUnsafe()
}

// checkRotationUnsafe checks if log rotation is needed (caller must hold mutex)
func (l *EnhancedLogger) checkRotationUnsafe() error {
	if l.file == nil || !l.config.Enabled {
		return nil
	}
	
	// Check file size
	maxSize := int64(l.config.MaxSizeMB) * 1024 * 1024
	if maxSize > 0 && l.fileSize >= maxSize {
		return l.rotateUnsafe()
	}
	
	// Check if date has changed (for daily rotation)
	currentFileName := generateLogFilename(l.config.FilenamePattern)
	if filepath.Base(l.fileName) != currentFileName {
		return l.rotateUnsafe()
	}
	
	return nil
}

// rotate performs log file rotation
func (l *EnhancedLogger) rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rotateUnsafe()
}

// rotateUnsafe performs log file rotation (caller must hold mutex)
func (l *EnhancedLogger) rotateUnsafe() error {
	// Close current file
	if l.file != nil {
		l.file.Close()
	}
	
	// Archive current file if it exists and has content
	if l.fileName != "" {
		if info, err := os.Stat(l.fileName); err == nil && info.Size() > 0 {
			// Create archived filename with timestamp
			dir := filepath.Dir(l.fileName) 
			base := filepath.Base(l.fileName)
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			timestamp := time.Now().Format("20060102-150405")
			archivedPath := filepath.Join(dir, fmt.Sprintf("%s-%s%s", name, timestamp, ext))
			
			// Move current file to archived location
			if err := os.Rename(l.fileName, archivedPath); err != nil {
				// If rename fails, try to continue anyway
				fmt.Fprintf(os.Stderr, "Failed to archive log file: %v\n", err)
			}
		}
	}
	
	// Open new file
	file, err := l.openLogFileUnsafe()
	if err != nil {
		return err
	}
	
	l.file = file
	
	// Update multi-writer
	writers := []io.Writer{}
	if l.config.ConsoleOutput {
		writers = append(writers, os.Stdout)
	}
	writers = append(writers, l.file)
	l.multiWriter = io.MultiWriter(writers...)
	
	// Recreate handler with new writer - use logger as writer for rotation
	handler := slog.NewTextHandler(l, &slog.HandlerOptions{
		Level: parseLogLevel(l.config.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Custom time format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02T15:04:05.000-07:00"))
			}
			// Shorten source paths for readability
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", filepath.Base(source.File), source.Line))
				}
			}
			return a
		},
	})
	l.Logger = slog.New(handler)
	
	// Clean old files if needed
	if l.config.MaxFiles > 0 {
		go l.cleanOldFiles()
	}
	
	return nil
}

// cleanOldFiles removes log files older than MaxFiles
func (l *EnhancedLogger) cleanOldFiles() {
	logDir := filepath.Dir(l.fileName)
	// Convert date patterns to glob wildcards for file matching
	pattern := strings.ReplaceAll(l.config.FilenamePattern, "YYYY", "*")
	pattern = strings.ReplaceAll(pattern, "YY", "*")
	pattern = strings.ReplaceAll(pattern, "MM", "*")
	pattern = strings.ReplaceAll(pattern, "M", "*")
	pattern = strings.ReplaceAll(pattern, "DD", "*")
	pattern = strings.ReplaceAll(pattern, "D", "*")
	pattern = strings.ReplaceAll(pattern, "HH", "*")
	pattern = strings.ReplaceAll(pattern, "H", "*")
	
	matches, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		return
	}
	
	// Sort by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: match, modTime: info.ModTime()})
	}
	
	// Sort newest first
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.Before(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	
	// Remove old files (only if MaxFiles is positive and we have excess files)
	if l.config.MaxFiles > 0 && len(files) > l.config.MaxFiles {
		for i := l.config.MaxFiles; i < len(files); i++ {
			os.Remove(files[i].path)
		}
	}
}

// Write implements io.Writer interface with rotation check
func (l *EnhancedLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	n, err = l.multiWriter.Write(p)
	if err != nil {
		return
	}
	
	l.fileSize += int64(n)
	
	// Check rotation after writing
	if err := l.checkRotationUnsafe(); err != nil {
		// Log rotation error but continue
		fmt.Fprintf(os.Stderr, "Log rotation error: %v\n", err)
	}
	
	return
}

// Close closes the log file
func (l *EnhancedLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogExecutionSummary logs a formatted execution summary for audit purposes
func (l *EnhancedLogger) LogExecutionSummary(startTime time.Time, configFile string, mode string, results []string, exitCode int) {
	duration := time.Since(startTime)
	
	l.Info("=== EXECUTION SUMMARY ===")
	l.Info("Execution details",
		slog.Time("start_time", startTime),
		slog.String("config_file", configFile),
		slog.String("mode", mode),
		slog.Duration("total_duration", duration),
		slog.Int("exit_code", exitCode))
	
	for _, result := range results {
		l.Info(result)
	}
}

// Backward compatibility functions - these now use the enhanced logger

// SetOutput configures dual output to console and file (deprecated - use Initialize with Config instead)
func SetOutput(filename string) error {
	config := Config{
		Enabled:         filename != "" && filename != "-",
		Directory:       filepath.Dir(filename),
		FilenamePattern: filepath.Base(filename),
		Level:           "info",
		MaxFiles:        0,
		MaxSizeMB:       0,
		ConsoleOutput:   true,
	}
	
	return Initialize(config)
}

// SetLevel sets the minimum logging level (deprecated - use Initialize with Config instead)
func SetLevel(level Level) {
	logger := Get()
	// This is a simplified implementation - for full level control, use Initialize with Config
	logger.Info("SetLevel called - for full control use Initialize with Config",
		slog.String("requested_level", fmt.Sprintf("%v", level)))
}

// Package-level logging functions - these use the global enhanced logger

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	Get().Debug(fmt.Sprintf(format, args...))
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	Get().Info(fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	Get().Warn(fmt.Sprintf(format, args...))
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	Get().Error(fmt.Sprintf(format, args...))
}

// Fatal logs a fatal message and exits
func Fatal(format string, args ...interface{}) {
	Get().Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Utility functions for common logging scenarios

// LogAPIRequest logs the start of an API request with structured fields
func LogAPIRequest(method, url string, headers map[string]string) {
	fields := []any{
		"method", method,
		"url", url,
		"type", "api_request",
	}

	// Add important headers (excluding sensitive ones)
	if userAgent := headers["User-Agent"]; userAgent != "" {
		fields = append(fields, "user_agent", userAgent)
	}

	Get().LogAttrs(context.Background(), slog.LevelInfo, "API request started", slog.Group("request", fields...))
}

// LogAPIResponse logs an API response with structured fields
func LogAPIResponse(method, url string, statusCode int, duration string, bodySize int) {
	level := slog.LevelInfo
	if statusCode >= 400 {
		level = slog.LevelWarn
	}
	if statusCode >= 500 {
		level = slog.LevelError
	}

	Get().LogAttrs(context.Background(), level, "API request completed",
		slog.Group("request",
			"method", method,
			"url", url,
			"status_code", statusCode,
			"duration", duration,
			"body_size", bodySize,
			"type", "api_response",
		),
	)
}

// LogFileOperation logs file operations with structured context
func LogFileOperation(operation, filepath string, size int64) {
	Get().LogAttrs(context.Background(), slog.LevelInfo, "File operation completed",
		slog.Group("file",
			"operation", operation,
			"path", filepath,
			"size_bytes", size,
			"type", "file_operation",
		),
	)
}

// LogFileError logs file operation errors with context
func LogFileError(operation, filePath string, err error) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		file = filepath.Base(file)
	}

	attrs := []slog.Attr{
		slog.Group("file",
			slog.String("operation", operation),
			slog.String("path", filePath),
			slog.String("type", "file_error"),
		),
		slog.String("error", err.Error()),
	}

	if ok {
		attrs = append(attrs, slog.String("source", fmt.Sprintf("%s:%d", file, line)))
	}

	Get().LogAttrs(context.Background(), slog.LevelError, "File operation failed", attrs...)
}

// LogOperationStart logs the beginning of an operation and returns a completion function
func LogOperationStart(operation string, details map[string]any) func(error) {
	startTime := time.Now()

	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("type", "operation_start"),
		slog.Time("start_time", startTime),
	}

	// Add details as structured attributes
	if details != nil {
		detailAttrs := make([]any, 0, len(details)*2)
		for k, v := range details {
			detailAttrs = append(detailAttrs, k, v)
		}
		attrs = append(attrs, slog.Group("details", detailAttrs...))
	}

	Get().LogAttrs(context.Background(), slog.LevelInfo, "Operation started", attrs...)

	// Return completion function
	return func(err error) {
		duration := time.Since(startTime)
		level := slog.LevelInfo
		message := "Operation completed"

		completionAttrs := []slog.Attr{
			slog.String("operation", operation),
			slog.String("type", "operation_complete"),
			slog.Duration("duration", duration),
			slog.Bool("success", err == nil),
		}

		if err != nil {
			level = slog.LevelError
			message = "Operation failed"
			completionAttrs = append(completionAttrs, slog.String("error", err.Error()))
		}

		Get().LogAttrs(context.Background(), level, message, completionAttrs...)
	}
}

// LogStructuredError logs an error with structured context information
func LogStructuredError(err error, ctxFields map[string]any) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		file = filepath.Base(file)
	}

	attrs := []slog.Attr{
		slog.String("error", err.Error()),
		slog.String("type", "structured_error"),
	}

	if ok {
		attrs = append(attrs, slog.String("source", fmt.Sprintf("%s:%d", file, line)))
	}

	// Add context as structured attributes
	if ctxFields != nil && len(ctxFields) > 0 {
		contextAttrs := make([]any, 0, len(ctxFields)*2)
		for k, v := range ctxFields {
			contextAttrs = append(contextAttrs, k, v)
		}
		attrs = append(attrs, slog.Group("context", contextAttrs...))
	}

	Get().LogAttrs(context.Background(), slog.LevelError, "Error occurred", attrs...)
}

// LogWithFields logs a message with custom structured fields
func LogWithFields(level Level, message string, fields map[string]any) {
	slogLevel := slog.Level(level)
	if level == FatalLevel {
		slogLevel = slog.LevelError
	}

	// Convert fields to slog attributes
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	Get().LogAttrs(context.Background(), slogLevel, message, attrs...)

	if level == FatalLevel {
		os.Exit(1)
	}
}

// ParseLevel converts a string to a log level
func ParseLevel(levelStr string) (Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}
