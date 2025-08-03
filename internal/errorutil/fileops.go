package errorutil

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
)

// FileError represents a file operation error with additional context
type FileError struct {
	Operation string // The operation that failed (e.g., "read", "write", "create")
	Path      string // The file path that was being accessed
	Size      int64  // File size (if applicable)
	Perm      os.FileMode // File permissions (if applicable)
	Underlying error // The underlying error
}

func (e *FileError) Error() string {
	return fmt.Sprintf("%s operation failed for %s: %v", e.Operation, e.Path, e.Underlying)
}

func (e *FileError) Unwrap() error {
	return e.Underlying
}

// NewFileError creates a new FileError with proper context
func NewFileError(operation, path string, err error) *FileError {
	fileErr := &FileError{
		Operation:  operation,
		Path:       path,
		Underlying: err,
	}

	// Try to get file info for additional context
	if info, statErr := os.Stat(path); statErr == nil {
		fileErr.Size = info.Size()
		fileErr.Perm = info.Mode()
	}

	return fileErr
}

// LogFileError logs a file error with appropriate structured context
func LogFileError(logger *slog.Logger, fileErr *FileError) *FileError {
	if logger == nil {
		return fileErr
	}

	attrs := []slog.Attr{
		slog.String("operation", fileErr.Operation),
		slog.String("file_path", fileErr.Path),
		slog.String("error", fileErr.Underlying.Error()),
		slog.String("error_type", getFileErrorType(fileErr.Underlying)),
	}

	if fileErr.Size > 0 {
		attrs = append(attrs, slog.Int64("file_size", fileErr.Size))
	}

	if fileErr.Perm != 0 {
		attrs = append(attrs, slog.String("file_permissions", fileErr.Perm.String()))
	}

	// Add directory information
	dir := filepath.Dir(fileErr.Path)
	if dir != "." {
		attrs = append(attrs, slog.String("directory", dir))
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	logger.Error("File operation failed", anyAttrs...)
	return fileErr
}

// getFileErrorType returns a human-readable error type classification
func getFileErrorType(err error) string {
	if err == nil {
		return "unknown"
	}

	// Check for common file error types
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return "file_not_found"
	}
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, os.ErrPermission) {
		return "permission_denied"
	}
	if errors.Is(err, fs.ErrExist) || errors.Is(err, os.ErrExist) {
		return "file_exists"
	}
	if errors.Is(err, syscall.ENOSPC) {
		return "no_space_left"
	}
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
		return "too_many_open_files"
	}

	// Check for path errors
	if pathErr, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("path_error_%s", pathErr.Op)
	}

	// Check for link errors
	if linkErr, ok := err.(*os.LinkError); ok {
		return fmt.Sprintf("link_error_%s", linkErr.Op)
	}

	return "generic_file_error"
}

// DirectoryError represents a directory operation error
type DirectoryError struct {
	Operation  string   // The operation that failed
	Path       string   // The directory path
	Contents   []string // Directory contents (if available)
	Underlying error    // The underlying error
}

func (e *DirectoryError) Error() string {
	return fmt.Sprintf("directory %s failed for %s: %v", e.Operation, e.Path, e.Underlying)
}

func (e *DirectoryError) Unwrap() error {
	return e.Underlying
}

// NewDirectoryError creates a new DirectoryError
func NewDirectoryError(operation, path string, err error) *DirectoryError {
	dirErr := &DirectoryError{
		Operation:  operation,
		Path:       path,
		Underlying: err,
	}

	// Try to read directory contents for context (if it's a read operation)
	if operation == "read" || operation == "list" {
		if entries, readErr := os.ReadDir(path); readErr == nil {
			dirErr.Contents = make([]string, len(entries))
			for i, entry := range entries {
				dirErr.Contents[i] = entry.Name()
			}
		}
	}

	return dirErr
}

// LogDirectoryError logs a directory error with structured context
func LogDirectoryError(logger *slog.Logger, dirErr *DirectoryError) *DirectoryError {
	if logger == nil {
		return dirErr
	}

	attrs := []slog.Attr{
		slog.String("operation", dirErr.Operation),
		slog.String("directory_path", dirErr.Path),
		slog.String("error", dirErr.Underlying.Error()),
		slog.String("error_type", getFileErrorType(dirErr.Underlying)),
	}

	if len(dirErr.Contents) > 0 {
		// Log first few entries for context
		maxEntries := 5
		contents := dirErr.Contents
		if len(contents) > maxEntries {
			contents = contents[:maxEntries]
		}
		attrs = append(attrs, slog.Any("directory_contents", contents))
		attrs = append(attrs, slog.Int("total_entries", len(dirErr.Contents)))
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	logger.Error("Directory operation failed", anyAttrs...)
	return dirErr
}

// EnsureDirectoryWithLogging ensures a directory exists, logging any errors
func EnsureDirectoryWithLogging(logger *slog.Logger, path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		dirErr := NewDirectoryError("create", path, err)
		LogDirectoryError(logger, dirErr)
		return dirErr
	}

	// Log successful directory creation
	if logger != nil {
		logger.Debug("Directory ensured",
			slog.String("directory_path", path),
			slog.String("permissions", perm.String()))
	}

	return nil
}

// SafeFileWrite performs a safe file write with logging and backup
func SafeFileWrite(logger *slog.Logger, path string, data []byte, perm os.FileMode) error {
	// Create backup if file exists
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".backup"
		if err := os.Rename(path, backupPath); err != nil {
			fileErr := NewFileError("backup", path, err)
			LogFileError(logger, fileErr)
			return fileErr
		}
		defer os.Remove(backupPath) // Clean up backup on success
	}

	// Write to temporary file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, perm); err != nil {
		fileErr := NewFileError("write_temp", tempPath, err)
		LogFileError(logger, fileErr)
		return fileErr
	}

	// Atomic move to final location
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file
		fileErr := NewFileError("move", path, err)
		LogFileError(logger, fileErr)
		return fileErr
	}

	// Log successful write
	if logger != nil {
		logger.Debug("File written successfully",
			slog.String("file_path", path),
			slog.Int("bytes_written", len(data)),
			slog.String("permissions", perm.String()))
	}

	return nil
}

// CleanupTempFiles removes temporary files matching a pattern
func CleanupTempFiles(logger *slog.Logger, dir, pattern string) error {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		dirErr := NewDirectoryError("glob", dir, err)
		LogDirectoryError(logger, dirErr)
		return dirErr
	}

	removed := 0
	var lastErr error

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			fileErr := NewFileError("remove", match, err)
			LogFileError(logger, fileErr)
			lastErr = fileErr
		} else {
			removed++
		}
	}

	// Log cleanup results
	if logger != nil {
		attrs := []slog.Attr{
			slog.String("directory", dir),
			slog.String("pattern", pattern),
			slog.Int("files_removed", removed),
			slog.Int("total_matches", len(matches)),
		}

		if lastErr != nil {
			attrs = append(attrs, slog.String("last_error", lastErr.Error()))
		}

		anyAttrs := make([]any, len(attrs))
		for i, attr := range attrs {
			anyAttrs[i] = attr
		}

		logger.Debug("Temporary file cleanup completed", anyAttrs...)
	}

	return lastErr
}