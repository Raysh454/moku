package tracker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FSStore implements content-addressed blob storage on the filesystem.
// Blobs are stored under siteDir/.moku/blobs using SHA-256 hash as the filename.
// The first two characters of the hash form a subdirectory to avoid too many files in one directory.
type FSStore struct {
	blobsDir string
}

// NewFSStore creates a new FSStore rooted at the given blobs directory.
func NewFSStore(blobsDir string) (*FSStore, error) {
	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blobs directory: %w", err)
	}
	return &FSStore{blobsDir: blobsDir}, nil
}

// Put stores content and returns its content-addressed ID (SHA-256 hex).
// If the content already exists, it returns the existing ID without rewriting.
func (fs *FSStore) Put(data []byte) (string, error) {
	// Compute SHA-256 hash
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// Check if blob already exists
	blobPath := fs.blobPath(hashStr)
	if _, err := os.Stat(blobPath); err == nil {
		// Blob already exists, return the hash
		return hashStr, nil
	}

	// Create subdirectory (first 2 chars of hash)
	subdir := filepath.Join(fs.blobsDir, hashStr[:2])
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return "", fmt.Errorf("failed to create blob subdirectory: %w", err)
	}

	// Write blob atomically using temp file + rename
	if err := AtomicWriteFile(blobPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write blob: %w", err)
	}

	return hashStr, nil
}

// Get retrieves content by its content-addressed ID.
func (fs *FSStore) Get(blobID string) ([]byte, error) {
	blobPath := fs.blobPath(blobID)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob not found: %s", blobID)
		}
		return nil, fmt.Errorf("failed to read blob: %w", err)
	}

	// Verify integrity by checking hash
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])
	if hashStr != blobID {
		return nil, fmt.Errorf("blob integrity check failed: expected %s, got %s", blobID, hashStr)
	}

	return data, nil
}

// Exists checks if a blob with the given ID exists.
func (fs *FSStore) Exists(blobID string) bool {
	blobPath := fs.blobPath(blobID)
	_, err := os.Stat(blobPath)
	return err == nil
}

// Delete removes a blob by its ID.
// This should only be called during garbage collection.
func (fs *FSStore) Delete(blobID string) error {
	blobPath := fs.blobPath(blobID)
	if err := os.Remove(blobPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete blob: %w", err)
	}
	return nil
}

// blobPath returns the filesystem path for a given blob ID.
// Format: blobsDir/{first2chars}/{fullhash}
func (fs *FSStore) blobPath(blobID string) string {
	// Validate blob ID length to prevent path traversal attacks
	// SHA-256 hex is always 64 characters
	if len(blobID) < 2 {
		// Return an invalid path that will cause operations to fail safely
		// Using a subdirectory name that can't match any real blob
		return filepath.Join(fs.blobsDir, "__invalid__", blobID)
	}
	return filepath.Join(fs.blobsDir, blobID[:2], blobID)
}

// GetReader returns a reader for a blob without loading it all into memory.
// Useful for large blobs.
func (fs *FSStore) GetReader(blobID string) (io.ReadCloser, error) {
	blobPath := fs.blobPath(blobID)
	file, err := os.Open(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob not found: %s", blobID)
		}
		return nil, fmt.Errorf("failed to open blob: %w", err)
	}
	return file, nil
}

// PutReader stores content from a reader and returns its content-addressed ID.
// This is more efficient for large content that doesn't need to fit in memory.
func (fs *FSStore) PutReader(reader io.Reader) (string, error) {
	// Create temp file to compute hash and store content
	tmpFile, err := os.CreateTemp(fs.blobsDir, ".tmp-blob-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	// Compute hash while copying to temp file
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, reader); err != nil {
		return "", fmt.Errorf("failed to copy data: %w", err)
	}

	// Sync and close temp file
	if err := tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Get hash
	hashStr := hex.EncodeToString(hasher.Sum(nil))
	blobPath := fs.blobPath(hashStr)

	// Check if blob already exists
	if _, err := os.Stat(blobPath); err == nil {
		// Blob already exists, just return the hash
		return hashStr, nil
	}

	// Create subdirectory
	subdir := filepath.Join(fs.blobsDir, hashStr[:2])
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return "", fmt.Errorf("failed to create blob subdirectory: %w", err)
	}

	// Set permissions and rename
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := os.Rename(tmpPath, blobPath); err != nil {
		return "", fmt.Errorf("failed to rename temp file: %w", err)
	}

	return hashStr, nil
}
