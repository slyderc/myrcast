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

var (
	// Default slog logger instance
	defaultLogger *slog.Logger

	// Current output writers for dual output
	consoleWriter io.Writer = os.Stdout
	fileWriter    io.Writer

	// Current minimum level
	currentLevel Level = InfoLevel
)

func init() {
	setupLogger()
}

// setupLogger initializes the default logger with console output
func setupLogger() {
	opts := &slog.HandlerOptions{
		Level:     slog.Level(currentLevel),
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize source attribute to show only filename:line
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					source.File = filepath.Base(source.File)
				}
			}
			return a
		},
	}

	// Create text handler for console output
	handler := slog.NewTextHandler(consoleWriter, opts)
	defaultLogger = slog.New(handler)
}

// multiHandler combines console and file output
type multiHandler struct {
	consoleHandler slog.Handler
	fileHandler    slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.Level(currentLevel)
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Write to console
	if h.consoleHandler != nil {
		if err := h.consoleHandler.Handle(ctx, r); err != nil {
			return err
		}
	}

	// Write to file if configured
	if h.fileHandler != nil {
		if err := h.fileHandler.Handle(ctx, r); err != nil {
			return err
		}
	}

	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &multiHandler{
		consoleHandler: h.consoleHandler.WithAttrs(attrs),
		fileHandler:    h.fileHandler.WithAttrs(attrs),
	}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	return &multiHandler{
		consoleHandler: h.consoleHandler.WithGroup(name),
		fileHandler:    h.fileHandler.WithGroup(name),
	}
}

// SetOutput configures dual output to console and file
func SetOutput(filename string) error {
	if filename == "" || filename == "-" {
		// Console only
		setupLogger()
		return nil
	}

	// Create log directory if needed (Windows compatible)
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file with appropriate permissions
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	fileWriter = file

	// Create handlers for both console and file
	opts := &slog.HandlerOptions{
		Level:     slog.Level(currentLevel),
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					source.File = filepath.Base(source.File)
				}
			}
			return a
		},
	}

	consoleHandler := slog.NewTextHandler(consoleWriter, opts)
	fileHandler := slog.NewTextHandler(fileWriter, opts)

	// Create multi-handler for dual output
	multiH := &multiHandler{
		consoleHandler: consoleHandler,
		fileHandler:    fileHandler,
	}

	defaultLogger = slog.New(multiH)
	return nil
}

// SetLevel sets the minimum logging level
func SetLevel(level Level) {
	currentLevel = level
	// Recreate logger with new level
	if fileWriter != nil {
		// Dual output mode
		opts := &slog.HandlerOptions{
			Level:     slog.Level(currentLevel),
			AddSource: true,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.SourceKey {
					if source, ok := a.Value.Any().(*slog.Source); ok {
						source.File = filepath.Base(source.File)
					}
				}
				return a
			},
		}

		consoleHandler := slog.NewTextHandler(consoleWriter, opts)
		fileHandler := slog.NewTextHandler(fileWriter, opts)

		multiH := &multiHandler{
			consoleHandler: consoleHandler,
			fileHandler:    fileHandler,
		}

		defaultLogger = slog.New(multiH)
	} else {
		// Console only
		setupLogger()
	}
}

// Package-level logging functions

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	logWithCaller(DebugLevel, fmt.Sprintf(format, args...))
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	logWithCaller(InfoLevel, fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	logWithCaller(WarnLevel, fmt.Sprintf(format, args...))
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	logWithCaller(ErrorLevel, fmt.Sprintf(format, args...))
}

// Fatal logs a fatal message and exits
func Fatal(format string, args ...interface{}) {
	logWithCaller(FatalLevel, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// logWithCaller logs a message with proper caller information
func logWithCaller(level Level, msg string) {
	if level < currentLevel {
		return
	}

	// Get caller information for context
	_, file, line, ok := runtime.Caller(2)
	if ok {
		file = filepath.Base(file)
	}

	// Convert our Level to slog.Level for logging
	slogLevel := slog.Level(level)
	if level == FatalLevel {
		slogLevel = slog.LevelError // Log as error since slog doesn't have fatal
	}

	// Log with source context
	if ok {
		defaultLogger.Log(nil, slogLevel, msg, "source", fmt.Sprintf("%s:%d", file, line))
	} else {
		defaultLogger.Log(nil, slogLevel, msg)
	}
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

	defaultLogger.LogAttrs(context.Background(), slog.LevelInfo, "API request started", slog.Group("request", fields...))
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

	defaultLogger.LogAttrs(context.Background(), level, "API request completed",
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
	defaultLogger.LogAttrs(context.Background(), slog.LevelInfo, "File operation completed",
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

	defaultLogger.LogAttrs(context.Background(), slog.LevelError, "File operation failed", attrs...)
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

	defaultLogger.LogAttrs(context.Background(), slog.LevelInfo, "Operation started", attrs...)

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

		defaultLogger.LogAttrs(context.Background(), level, message, completionAttrs...)
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

	defaultLogger.LogAttrs(context.Background(), slog.LevelError, "Error occurred", attrs...)
}

// LogWithFields logs a message with custom structured fields
func LogWithFields(level Level, message string, fields map[string]any) {
	if level < currentLevel {
		return
	}

	slogLevel := slog.Level(level)
	if level == FatalLevel {
		slogLevel = slog.LevelError
	}

	// Convert fields to slog attributes
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	defaultLogger.LogAttrs(context.Background(), slogLevel, message, attrs...)

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
