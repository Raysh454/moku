package tracker_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/model"
	"github.com/raysh454/moku/internal/tracker"
)

func TestNewInMemoryTracker_Constructable(t *testing.T) {
	t.Parallel()
	cfg := &tracker.Config{}
	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewInMemoryTracker(cfg, logger)
	if err != nil {
		t.Fatalf("NewInMemoryTracker returned error: %v", err)
	}
	defer tr.Close()

	// Methods return ErrNotImplemented for now — assert that behavior so tests are explicit.
	_, err = tr.Commit(context.Background(), nil, "msg", "author")
	if err == nil {
		t.Fatalf("expected ErrNotImplemented from Commit, got nil")
	}
	if !errors.Is(err, tracker.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestNewSQLiteTracker_Constructable(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	// Verify .moku directory was created
	mokuDir := filepath.Join(tmpDir, ".moku")
	if _, err := os.Stat(mokuDir); os.IsNotExist(err) {
		t.Errorf(".moku directory was not created")
	}

	// Verify database was created
	dbPath := filepath.Join(mokuDir, "moku.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file was not created")
	}

	// Verify blobs directory was created
	blobsDir := filepath.Join(mokuDir, "blobs")
	if _, err := os.Stat(blobsDir); os.IsNotExist(err) {
		t.Errorf("blobs directory was not created")
	}
}

func TestSQLiteTracker_CommitAndGet(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	// Create a test snapshot
	snapshot := &model.Snapshot{
		URL:        "https://example.com",
		StatusCode: 200,
		Body:       []byte("<html><body>Hello World</body></html>"),
		Headers: map[string][]string{
			"Content-Type": []string{"text/html"},
		},
	}

	// Commit the snapshot
	ctx := context.Background()
	version, err := tr.Commit(ctx, snapshot, "Initial commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if version == nil {
		t.Fatal("Commit returned nil version")
	}

	if version.ID == "" {
		t.Error("version ID is empty")
	}

	if version.Message != "Initial commit" {
		t.Errorf("expected message 'Initial commit', got %q", version.Message)
	}

	if version.Author != "test@example.com" {
		t.Errorf("expected author 'test@example.com', got %q", version.Author)
	}

	// Get the snapshot back
	retrievedSnapshot, err := tr.Get(ctx, version.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if retrievedSnapshot == nil {
		t.Fatal("Get returned nil snapshot")
	}

	if retrievedSnapshot.URL != snapshot.URL {
		t.Errorf("expected URL %q, got %q", snapshot.URL, retrievedSnapshot.URL)
	}

	if string(retrievedSnapshot.Body) != string(snapshot.Body) {
		t.Errorf("body mismatch: expected %q, got %q", string(snapshot.Body), string(retrievedSnapshot.Body))
	}
}

func TestSQLiteTracker_List(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// List should be empty initially
	versions, err := tr.List(ctx, 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}

	// Commit a few snapshots
	for i := 0; i < 3; i++ {
		snapshot := &model.Snapshot{
			URL:  "https://example.com",
			Body: []byte(fmt.Sprintf("<html><body>Version %d</body></html>", i)),
		}
		_, err := tr.Commit(ctx, snapshot, fmt.Sprintf("Commit %d", i), "test@example.com")
		if err != nil {
			t.Fatalf("Commit %d returned error: %v", i, err)
		}
	}

	// List should return all versions
	versions, err = tr.List(ctx, 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Versions should be in reverse chronological order (newest first)
	for i := 0; i < len(versions)-1; i++ {
		if versions[i].Timestamp.Before(versions[i+1].Timestamp) {
			t.Errorf("versions not in reverse chronological order")
		}
	}
}

func TestSQLiteTracker_Diff(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Commit first version
	snapshot1 := &model.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Version 1</body></html>"),
	}
	version1, err := tr.Commit(ctx, snapshot1, "First commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 1 returned error: %v", err)
	}

	// Commit second version
	snapshot2 := &model.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Version 2</body></html>"),
	}
	version2, err := tr.Commit(ctx, snapshot2, "Second commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 2 returned error: %v", err)
	}

	// Compute diff
	diff, err := tr.Diff(ctx, version1.ID, version2.ID)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if diff == nil {
		t.Fatal("Diff returned nil")
	}

	if diff.BaseID != version1.ID {
		t.Errorf("expected BaseID %q, got %q", version1.ID, diff.BaseID)
	}

	if diff.HeadID != version2.ID {
		t.Errorf("expected HeadID %q, got %q", version2.ID, diff.HeadID)
	}

	// Should have at least one chunk (actual diff implementation)
	if len(diff.Chunks) == 0 {
		t.Error("expected at least one diff chunk")
	}

	// Verify diff chunks make sense (version 1 removed, version 2 added)
	hasRemoved := false
	hasAdded := false
	for _, chunk := range diff.Chunks {
		if chunk.Type == "removed" && chunk.Content == "1" {
			hasRemoved = true
		}
		if chunk.Type == "added" && chunk.Content == "2" {
			hasAdded = true
		}
	}
	if !hasRemoved || !hasAdded {
		t.Errorf("expected diff to show version 1 removed and version 2 added, got chunks: %+v", diff.Chunks)
	}
}

func TestSQLiteTracker_Checkout(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Commit first version
	snapshot1 := &model.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>First Version</body></html>"),
	}
	version1, err := tr.Commit(ctx, snapshot1, "First commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 1 returned error: %v", err)
	}

	// Commit second version
	snapshot2 := &model.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Second Version</body></html>"),
	}
	version2, err := tr.Commit(ctx, snapshot2, "Second commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 2 returned error: %v", err)
	}

	// Checkout first version
	err = tr.Checkout(ctx, version1.ID)
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	// Verify the working-tree file was restored
	// Files are now written as index.html/.page_body
	indexPath := filepath.Join(tmpDir, ".page_body")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read checked out file: %v", err)
	}

	expected := "<html><body>First Version</body></html>"
	if string(content) != expected {
		t.Errorf("expected content %q, got %q", expected, string(content))
	}

	// Also verify headers file was created
	headersPath := filepath.Join(tmpDir, ".page_headers.json")
	if _, err := os.Stat(headersPath); os.IsNotExist(err) {
		t.Error(".page_headers.json was not created")
	}

	// Checkout second version
	err = tr.Checkout(ctx, version2.ID)
	if err != nil {
		t.Fatalf("Checkout of version 2 returned error: %v", err)
	}

	// Verify the file was updated
	content, err = os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read checked out file: %v", err)
	}

	expected = "<html><body>Second Version</body></html>"
	if string(content) != expected {
		t.Errorf("expected content %q, got %q", expected, string(content))
	}

	// Verify HEAD was updated
	headPath := filepath.Join(tmpDir, ".moku", "HEAD")
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}

	if string(headContent) != version2.ID {
		t.Errorf("expected HEAD to be %q, got %q", version2.ID, string(headContent))
	}
}
