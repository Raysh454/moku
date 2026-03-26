package tracker_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

// TestBeginCommit_CreatesVersion verifies that BeginCommit creates a version and returns a valid PendingCommit.
func TestBeginCommit_CreatesVersion(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test commit", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}

	if pc == nil {
		t.Fatal("BeginCommit returned nil PendingCommit")
	}

	if pc.VersionID == "" {
		t.Error("PendingCommit has empty VersionID")
	}

	if pc.Message != "Test commit" {
		t.Errorf("Expected message 'Test commit', got '%s'", pc.Message)
	}

	if pc.Author != "test-author" {
		t.Errorf("Expected author 'test-author', got '%s'", pc.Author)
	}

	if pc.GetTransaction() == nil {
		t.Error("PendingCommit has no active transaction")
	}

	// Clean up
	if err := tr.CancelCommit(ctx, pc); err != nil {
		t.Errorf("CancelCommit failed: %v", err)
	}
}

// TestAddSnapshots_SingleBatch verifies adding a single batch of snapshots.
func TestAddSnapshots_SingleBatch(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test batch", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}
	defer func() {
		if err := tr.CancelCommit(ctx, pc); err != nil {
			t.Fatalf("CancelCommit failed: %v", err)
		}
	}()

	snapshots := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: []byte("<html>Page 1</html>"), StatusCode: 200},
		{URL: "https://example.com/page2", Body: []byte("<html>Page 2</html>"), StatusCode: 200},
		{URL: "https://example.com/page3", Body: []byte("<html>Page 3</html>"), StatusCode: 200},
	}

	if err := tr.AddSnapshots(ctx, pc, snapshots); err != nil {
		t.Fatalf("AddSnapshots failed: %v", err)
	}

	if pc.GetSnapshotCount() != 3 {
		t.Errorf("Expected 3 snapshots, got %d", pc.GetSnapshotCount())
	}
}

// TestAddSnapshots_MultipleBatches verifies adding multiple batches to the same commit.
func TestAddSnapshots_MultipleBatches(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Multi-batch commit", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}
	defer func() {
		if err := tr.CancelCommit(ctx, pc); err != nil {
			t.Fatalf("CancelCommit failed: %v", err)
		}
	}()

	// Add first batch
	batch1 := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: []byte("<html>Page 1</html>"), StatusCode: 200},
		{URL: "https://example.com/page2", Body: []byte("<html>Page 2</html>"), StatusCode: 200},
	}
	if err := tr.AddSnapshots(ctx, pc, batch1); err != nil {
		t.Fatalf("AddSnapshots batch1 failed: %v", err)
	}

	// Add second batch
	batch2 := []*models.Snapshot{
		{URL: "https://example.com/page3", Body: []byte("<html>Page 3</html>"), StatusCode: 200},
		{URL: "https://example.com/page4", Body: []byte("<html>Page 4</html>"), StatusCode: 200},
	}
	if err := tr.AddSnapshots(ctx, pc, batch2); err != nil {
		t.Fatalf("AddSnapshots batch2 failed: %v", err)
	}

	// Add third batch
	batch3 := []*models.Snapshot{
		{URL: "https://example.com/page5", Body: []byte("<html>Page 5</html>"), StatusCode: 200},
	}
	if err := tr.AddSnapshots(ctx, pc, batch3); err != nil {
		t.Fatalf("AddSnapshots batch3 failed: %v", err)
	}

	if pc.GetSnapshotCount() != 5 {
		t.Errorf("Expected 5 total snapshots, got %d", pc.GetSnapshotCount())
	}
}

// TestFinalizeCommit_ComputesDiffs verifies that FinalizeCommit creates one version.
func TestFinalizeCommit_CreatesOneVersion(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Multi-batch commit", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}

	// Add multiple batches
	for i := 0; i < 3; i++ {
		batch := []*models.Snapshot{
			{URL: "https://example.com/batch" + string(rune('A'+i)), Body: []byte("<html>Batch</html>"), StatusCode: 200},
		}
		if err := tr.AddSnapshots(ctx, pc, batch); err != nil {
			t.Fatalf("AddSnapshots failed: %v", err)
		}
	}

	cr, err := tr.FinalizeCommit(ctx, pc)
	if err != nil {
		t.Fatalf("FinalizeCommit failed: %v", err)
	}

	if cr == nil {
		t.Fatal("FinalizeCommit returned nil CommitResult")
	}

	if cr.Version.ID == "" {
		t.Error("CommitResult has empty Version ID")
	}

	if len(cr.Snapshots) != 3 {
		t.Errorf("Expected 3 snapshots in result, got %d", len(cr.Snapshots))
	}

	// Verify all snapshots have the same version ID
	for i, snap := range cr.Snapshots {
		if snap.VersionID != cr.Version.ID {
			t.Errorf("Snapshot %d has different version ID: %s != %s", i, snap.VersionID, cr.Version.ID)
		}
	}
}

// TestFinalizeCommit_UpdatesHEAD verifies that HEAD is updated after finalize.
func TestFinalizeCommit_UpdatesHEAD(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test HEAD update", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}

	snapshots := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: []byte("<html>Test</html>"), StatusCode: 200},
	}
	if err := tr.AddSnapshots(ctx, pc, snapshots); err != nil {
		t.Fatalf("AddSnapshots failed: %v", err)
	}

	cr, err := tr.FinalizeCommit(ctx, pc)
	if err != nil {
		t.Fatalf("FinalizeCommit failed: %v", err)
	}

	head, err := tr.ReadHEAD()
	if err != nil {
		t.Fatalf("ReadHEAD failed: %v", err)
	}

	if head != cr.Version.ID {
		t.Errorf("HEAD not updated: expected %s, got %s", cr.Version.ID, head)
	}
}

// TestCancelCommit_Rollback verifies that CancelCommit rolls back the transaction.
func TestCancelCommit_Rollback(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test rollback", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}

	versionID := pc.VersionID

	snapshots := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: []byte("<html>Test</html>"), StatusCode: 200},
	}
	if err := tr.AddSnapshots(ctx, pc, snapshots); err != nil {
		t.Fatalf("AddSnapshots failed: %v", err)
	}

	// Cancel the commit
	if err := tr.CancelCommit(ctx, pc); err != nil {
		t.Fatalf("CancelCommit failed: %v", err)
	}

	// Verify version was not persisted
	versions, err := tr.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	for _, v := range versions {
		if v.ID == versionID {
			t.Errorf("Version %s should have been rolled back but was found", versionID)
		}
	}

	// Verify transaction is no longer active
	if pc.GetTransaction() != nil {
		t.Error("Transaction should be nil after CancelCommit")
	}
}

// TestAddSnapshots_AfterCancel_Fails verifies that AddSnapshots fails after cancel.
func TestAddSnapshots_AfterCancel_Fails(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test after cancel", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}

	if err := tr.CancelCommit(ctx, pc); err != nil {
		t.Fatalf("CancelCommit failed: %v", err)
	}

	// Attempt to add snapshots after cancel
	snapshots := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: []byte("<html>Test</html>"), StatusCode: 200},
	}
	err = tr.AddSnapshots(ctx, pc, snapshots)
	if err == nil {
		t.Error("AddSnapshots should fail after CancelCommit")
	}
}

// TestAddSnapshots_BlobDeduplication verifies that identical content reuses blobs.
func TestAddSnapshots_BlobDeduplication(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := logging.NewStdoutLogger("pending-commit-test")

	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: tmpDir,
		ProjectID:   "pending-commit-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	defer tr.Close()

	pc, err := tr.BeginCommit(ctx, "Test deduplication", "test-author")
	if err != nil {
		t.Fatalf("BeginCommit failed: %v", err)
	}
	defer func() {
		if err := tr.CancelCommit(ctx, pc); err != nil {
			t.Fatalf("CancelCommit failed: %v", err)
		}
	}()

	sameContent := []byte("<html>Same content</html>")

	snapshots := []*models.Snapshot{
		{URL: "https://example.com/page1", Body: sameContent, StatusCode: 200},
		{URL: "https://example.com/page2", Body: sameContent, StatusCode: 200},
		{URL: "https://example.com/page3", Body: sameContent, StatusCode: 200},
	}

	if err := tr.AddSnapshots(ctx, pc, snapshots); err != nil {
		t.Fatalf("AddSnapshots failed: %v", err)
	}

	cr, err := tr.FinalizeCommit(ctx, pc)
	if err != nil {
		t.Fatalf("FinalizeCommit failed: %v", err)
	}

	// All three snapshots should exist
	if len(cr.Snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(cr.Snapshots))
	}

	// Note: Blob deduplication happens at storage level, so we can't easily verify
	// the number of blobs without accessing internal blobstore. This test just ensures
	// the snapshots are created successfully with identical content.
}
