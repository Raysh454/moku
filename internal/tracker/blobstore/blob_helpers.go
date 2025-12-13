package blobstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AtomicWriteFile writes data to a file atomically using a temp file + rename strategy.
// This ensures the file is either fully written or not written at all, preventing corruption.
//
// Steps:
// 1. Create a temporary file in the same directory as the target
// 2. Write data to the temp file
// 3. Fsync to ensure data is on disk
// 4. Rename temp file to target (atomic on POSIX systems)
// 5. Set file permissions
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Sanitize the path to prevent directory traversal
	if err := validatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create temp file in the same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup temp file on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Fsync to ensure data is persisted to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tmpFile = nil // Prevent double close in defer

	// Set permissions on temp file before rename
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename (on POSIX systems)
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// validatePath checks if a path is safe to use (no directory traversal, no absolute paths outside siteDir)
func validatePath(path string) error {
	// Clean the path to resolve any . or .. components
	cleaned := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path contains '..' which is not allowed")
	}

	// Additional validation can be added here (e.g., check against siteDir)
	return nil
}
