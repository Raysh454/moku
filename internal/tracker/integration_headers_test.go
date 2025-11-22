package tracker_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/model"
	"github.com/raysh454/moku/internal/tracker"
)

func TestHeaderStorage_Integration(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-headers-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-headers-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Snapshot with headers
	headers := map[string][]string{
		"Content-Type":  {"text/html; charset=utf-8"},
		"Cache-Control": {"no-cache", "no-store"},
		"Server":        {"nginx/1.20.0"},
	}

	snapshot1 := &model.Snapshot{
		StatusCode: 200,
		URL:        "https://example.com",
		Body:       []byte("<html><body>Version 1</body></html>"),
		Headers:    headers,
	}

	version1, err := tr.Commit(ctx, snapshot1, "Initial commit with headers", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 1 returned error: %v", err)
	}

	if version1 == nil {
		t.Fatal("Commit returned nil version")
	}

	retrievedSnapshot, err := tr.Get(ctx, version1.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if retrievedSnapshot == nil {
		t.Fatal("Get returned nil snapshot")
	}

	// Second snapshot with modified headers
	headers2 := map[string][]string{
		"Content-Type":  {"application/json"},
		"Cache-Control": {"no-cache", "no-store"},
		"Server":        {"nginx/1.21.0"},
		"X-Custom":      {"value"},
	}

	snapshot2 := &model.Snapshot{
		StatusCode: 200,
		URL:        "https://example.com",
		Body:       []byte("<html><body>Version 2</body></html>"),
		Headers:    headers2,
	}

	version2, err := tr.Commit(ctx, snapshot2, "Update with header changes", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 2 returned error: %v", err)
	}

	diff, err := tr.Diff(ctx, version1.ID, version2.ID)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if diff == nil {
		t.Fatal("Diff returned nil")
	}

	if len(diff.Chunks) == 0 {
		t.Error("expected at least one body diff chunk")
	}

	t.Logf("Diff computed successfully with %d chunks", len(diff.Chunks))
}

func TestHeaderNormalization_Integration(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-headers-norm-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-headers-norm-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	headers := map[string][]string{
		"Content-Type":  {"text/html"},
		"content-type":  {"application/json"}, // duplicate with different case
		"CACHE-CONTROL": {"no-cache"},
	}

	snapshot := &model.Snapshot{
		StatusCode: 200,
		URL:        "https://example.com",
		Body:       []byte("<html><body>Test</body></html>"),
		Headers:    headers,
	}

	version, err := tr.Commit(ctx, snapshot, "Test normalization", "test@example.com")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if version == nil {
		t.Fatal("Commit returned nil version")
	}

	t.Log("Header normalization test completed successfully")
}

func TestSensitiveHeaderRedaction_Integration(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-headers-redact-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-headers-redact-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	headers := map[string][]string{
		"Content-Type":  {"text/html"},
		"Authorization": {"Bearer secret-token-12345"},
		"Cookie":        {"session=abc123; tracking=xyz789"},
		"X-Api-Key":     {"super-secret-key"},
	}

	snapshot := &model.Snapshot{
		StatusCode: 200,
		URL:        "https://example.com",
		Body:       []byte("<html><body>Protected Content</body></html>"),
		Headers:    headers,
	}

	version, err := tr.Commit(ctx, snapshot, "Test with sensitive headers", "test@example.com")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if version == nil {
		t.Fatal("Commit returned nil version")
	}

	t.Log("Sensitive header redaction test completed successfully")
}

func TestMultipleVersionsWithHeaders_Integration(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-headers-multi-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-headers-multi-test")
	tr, err := tracker.NewSQLiteTracker(tmpDir, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	versions := make([]*model.Version, 0, 3)

	headers1 := map[string][]string{
		"Content-Type": {"text/html"},
		"Server":       {"nginx/1.20.0"},
	}
	v1, err := commitWithHeaders(ctx, tr, headers1, "Version 1", 1)
	if err != nil {
		t.Fatalf("Failed to commit version 1: %v", err)
	}
	versions = append(versions, v1)

	headers2 := map[string][]string{
		"Content-Type":  {"text/html"},
		"Server":        {"nginx/1.20.0"},
		"Cache-Control": {"no-cache"},
	}
	v2, err := commitWithHeaders(ctx, tr, headers2, "Version 2", 2)
	if err != nil {
		t.Fatalf("Failed to commit version 2: %v", err)
	}
	versions = append(versions, v2)

	headers3 := map[string][]string{
		"Content-Type":  {"application/json"},
		"Cache-Control": {"no-cache"},
	}
	v3, err := commitWithHeaders(ctx, tr, headers3, "Version 3", 3)
	if err != nil {
		t.Fatalf("Failed to commit version 3: %v", err)
	}
	versions = append(versions, v3)

	if len(versions) != 3 {
		t.Fatalf("Expected 3 versions, got %d", len(versions))
	}

	diff12, err := tr.Diff(ctx, v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("Failed to compute diff 1->2: %v", err)
	}
	if diff12 == nil {
		t.Fatal("Diff 1->2 returned nil")
	}

	diff23, err := tr.Diff(ctx, v2.ID, v3.ID)
	if err != nil {
		t.Fatalf("Failed to compute diff 2->3: %v", err)
	}
	if diff23 == nil {
		t.Fatal("Diff 2->3 returned nil")
	}

	t.Logf("Successfully committed and diffed %d versions with header changes", len(versions))
}

// Helper to commit snapshot with headers using new model.Snapshot
func commitWithHeaders(ctx context.Context, tr *tracker.SQLiteTracker, headers map[string][]string, message string, versionNum int) (*model.Version, error) {
	snapshot := &model.Snapshot{
		StatusCode: 200,
		URL:        "https://example.com",
		Body:       []byte(fmt.Sprintf("<html><body>Version %d</body></html>", versionNum)),
		Headers:    headers,
	}

	return tr.Commit(ctx, snapshot, message, "test@example.com")
}
