package tracker_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

func TestNewSQLiteTracker_Constructable(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "tracker-unit-test"}, logger, nil)
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
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "tracker-unit-test"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	// Create a test snapshot
	snapshot := &models.Snapshot{
		URL:        "https://example.com",
		StatusCode: 200,
		Body:       []byte("<html><body>Hello World</body></html>"),
		Headers: map[string][]string{
			"Content-Type": {"text/html"},
		},
	}

	// Commit the snapshot
	ctx := context.Background()
	commitResult, err := tr.Commit(ctx, snapshot, "Initial commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	if commitResult == nil {
		t.Fatal("Commit returned nil result")
	} else {
		version := &commitResult.Version
		if version.ID == "" {
			t.Error("version ID is empty")
		}

		if version.Message != "Initial commit" {
			t.Errorf("expected message 'Initial commit', got %q", version.Message)
		}

		if version.Author != "test@example.com" {
			t.Errorf("expected author 'test@example.com', got %q", version.Author)
		}

		// Get the snapshots back
		retrievedSnapshots, err := tr.GetSnapshots(ctx, version.ID)
		if err != nil {
			t.Fatalf("GetSnapshots returned error: %v", err)
		}

		if len(retrievedSnapshots) == 0 {
			t.Fatal("GetSnapshots returned no snapshots")
		}

		retrievedSnapshot := retrievedSnapshots[0]
		if retrievedSnapshot.URL != snapshot.URL {
			t.Errorf("expected URL %q, got %q", snapshot.URL, retrievedSnapshot.URL)
		}

		if string(retrievedSnapshot.Body) != string(snapshot.Body) {
			t.Errorf("body mismatch: expected %q, got %q", string(snapshot.Body), string(retrievedSnapshot.Body))
		}
	}
}

func TestSQLiteTracker_FinalizeEmptyCommitFails(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "test-project",
	}
	tr, err := tracker.NewSQLiteTracker(cfg, logging.NewStdoutLogger("tracker-test"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	ctx := context.Background()
	pc, err := tr.BeginCommit(ctx, "Empty commit", "tester")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tr.FinalizeCommit(ctx, pc)
	if err == nil {
		t.Error("expected error when finalizing empty commit, but got nil")
	}

	// Verify HEAD wasn't updated
	headExists, err := tr.HEADExists()
	if err != nil {
		t.Fatal(err)
	}
	if headExists {
		t.Error("expected HEAD to not exist for empty commit")
	}

	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) > 0 {
		t.Errorf("expected 0 versions, but found %d", len(versions))
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
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "tracker-unit-test"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// List should be empty initially
	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}

	// Commit a few snapshots
	for i := 0; i < 3; i++ {
		snapshot := &models.Snapshot{
			URL:  "https://example.com",
			Body: []byte(fmt.Sprintf("<html><body>Version %d</body></html>", i)),
		}
		_, err := tr.Commit(ctx, snapshot, fmt.Sprintf("Commit %d", i), "test@example.com")
		if err != nil {
			t.Fatalf("Commit %d returned error: %v", i, err)
		}
	}

	// List should return all versions
	versions, err = tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
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
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "tracker-unit-test"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Commit first version
	snapshot1 := &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Version 1</body></html>"),
	}
	result1, err := tr.Commit(ctx, snapshot1, "First commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 1 returned error: %v", err)
	}

	// Commit second version
	snapshot2 := &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Version 2</body></html>"),
	}
	result2, err := tr.Commit(ctx, snapshot2, "Second commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 2 returned error: %v", err)
	}

	// Compute diff
	diff, err := tr.DiffVersions(ctx, result1.Version.ID, result2.Version.ID)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if diff == nil {
		t.Fatal("Diff returned nil")
	} else {
		if diff.BaseVersionID != result1.Version.ID {
			t.Errorf("expected BaseVersionID %q, got %q", result1.Version.ID, diff.BaseVersionID)
		}

		if diff.HeadVersionID != result2.Version.ID {
			t.Errorf("expected HeadVersionID %q, got %q", result2.Version.ID, diff.HeadVersionID)
		}

		// Should have at least one chunk (actual diff implementation)
		chunkCount := 0
		for _, f := range diff.Files {
			chunkCount += len(f.BodyDiff.Chunks)
		}
		if chunkCount == 0 {
			t.Error("expected at least one diff chunk")
		}

		// Verify diff chunks make sense (version 1 removed, version 2 added)
		hasRemoved := false
		hasAdded := false
		for _, f := range diff.Files {
			for _, chunk := range f.BodyDiff.Chunks {
				if chunk.Type == "removed" && chunk.Content == "1" {
					hasRemoved = true
				}
				if chunk.Type == "added" && chunk.Content == "2" {
					hasAdded = true
				}
			}
		}
		if !hasRemoved || !hasAdded {
			t.Errorf("expected diff to show version 1 removed and version 2 added, got multi: %+v", diff)
		}
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
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "tracker-unit-test"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Commit first version
	snapshot1 := &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>First Version</body></html>"),
	}
	result1, err := tr.Commit(ctx, snapshot1, "First commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 1 returned error: %v", err)
	}

	// Commit second version
	snapshot2 := &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>Second Version</body></html>"),
	}
	result2, err := tr.Commit(ctx, snapshot2, "Second commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit 2 returned error: %v", err)
	}

	// Checkout first version
	err = tr.Checkout(ctx, result1.Version.ID)
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
	err = tr.Checkout(ctx, result2.Version.ID)
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

	if string(headContent) != result2.Version.ID {
		t.Errorf("expected HEAD to be %q, got %q", result2.Version.ID, string(headContent))
	}
}

func TestListVersions_EmptyTracker(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "test-proj"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestListVersions_SingleVersion(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "test-proj"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	snapshot := &models.Snapshot{
		URL:  "https://example.com/page",
		Body: []byte("<html>Version 1</html>"),
	}
	result, err := tr.Commit(ctx, snapshot, "Initial commit", "test@example.com")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	v := versions[0]
	if v.ID != result.Version.ID {
		t.Errorf("expected version ID %q, got %q", result.Version.ID, v.ID)
	}
	if v.Message != "Initial commit" {
		t.Errorf("expected message %q, got %q", "Initial commit", v.Message)
	}
	if v.Author != "test@example.com" {
		t.Errorf("expected author %q, got %q", "test@example.com", v.Author)
	}
	if v.Parent != "" {
		t.Errorf("expected empty parent for first version, got %q", v.Parent)
	}
}

func TestListVersions_MultipleVersionsOrdering(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "test-proj"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Create 5 versions with explicit timestamps
	var versionIDs []string
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		snapshot := &models.Snapshot{
			URL:       fmt.Sprintf("https://example.com/page%d", i),
			Body:      []byte(fmt.Sprintf("<html>Version %d</html>", i)),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
		result, err := tr.Commit(ctx, snapshot, fmt.Sprintf("Commit %d", i), "test@example.com")
		if err != nil {
			t.Fatalf("Commit %d returned error: %v", i, err)
		}
		versionIDs = append(versionIDs, result.Version.ID)
	}

	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 5 {
		t.Fatalf("expected 5 versions, got %d", len(versions))
	}

	// Verify reverse chronological order (newest first)
	for i := 0; i < len(versions)-1; i++ {
		if versions[i].Timestamp.Before(versions[i+1].Timestamp) {
			t.Errorf("versions not in reverse chronological order at index %d", i)
		}
	}

	// Verify newest version is first
	if versions[0].ID != versionIDs[4] {
		t.Errorf("expected newest version ID %q first, got %q", versionIDs[4], versions[0].ID)
	}

	// Verify oldest version is last
	if versions[4].ID != versionIDs[0] {
		t.Errorf("expected oldest version ID %q last, got %q", versionIDs[0], versions[4].ID)
	}
}

func TestListVersions_LimitRespected(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "test-proj"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Create 10 versions
	for i := 0; i < 10; i++ {
		snapshot := &models.Snapshot{
			URL:  fmt.Sprintf("https://example.com/page%d", i),
			Body: []byte(fmt.Sprintf("<html>Version %d</html>", i)),
		}
		_, err := tr.Commit(ctx, snapshot, fmt.Sprintf("Commit %d", i), "test@example.com")
		if err != nil {
			t.Fatalf("Commit %d returned error: %v", i, err)
		}
	}

	// Test limit of 3
	versions, err := tr.ListVersions(ctx, 3)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected limit of 3 to be respected, got %d versions", len(versions))
	}

	// Test limit of 0 (should return all)
	versions, err = tr.ListVersions(ctx, 0)
	if err != nil {
		t.Fatalf("ListVersions with limit 0 returned error: %v", err)
	}
	if len(versions) != 10 {
		t.Errorf("expected all 10 versions with limit 0, got %d", len(versions))
	}
}

func TestListVersions_ParentChains(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "moku-tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: tmpDir, ProjectID: "test-proj"}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// Create a chain of versions with explicit timestamps
	var prevVersionID string
	baseTime := time.Now()
	for i := 0; i < 3; i++ {
		snapshot := &models.Snapshot{
			URL:       "https://example.com/page",
			Body:      []byte(fmt.Sprintf("<html>Version %d</html>", i)),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
		result, err := tr.Commit(ctx, snapshot, fmt.Sprintf("Commit %d", i), "test@example.com")
		if err != nil {
			t.Fatalf("Commit %d returned error: %v", i, err)
		}

		if i > 0 {
			// Verify parent is set correctly
			if result.Version.Parent != prevVersionID {
				t.Errorf("version %d: expected parent %q, got %q", i, prevVersionID, result.Version.Parent)
			}
		} else {
			// First version should have no parent
			if result.Version.Parent != "" {
				t.Errorf("version 0: expected empty parent, got %q", result.Version.Parent)
			}
		}
		prevVersionID = result.Version.ID
	}

	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// Verify parent chain (newest to oldest)
	// versions[0] is newest, should have parent versions[1]
	// versions[1] is middle, should have parent versions[2]
	// versions[2] is oldest, should have no parent
	if versions[0].Parent != versions[1].ID {
		t.Errorf("version 0 (newest) parent mismatch: expected %q, got %q", versions[1].ID, versions[0].Parent)
	}
	if versions[1].Parent != versions[2].ID {
		t.Errorf("version 1 (middle) parent mismatch: expected %q, got %q", versions[2].ID, versions[1].Parent)
	}
	if versions[2].Parent != "" {
		t.Errorf("version 2 (oldest) should have no parent, got %q", versions[2].Parent)
	}
}
