package tracker_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

func newTestTracker(t *testing.T) (tracker.Tracker, string) {
	t.Helper()
	dir := t.TempDir()
	logger := logging.NewStdoutLogger("tracker-extended-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath: dir,
		ProjectID:   "extended-test",
	}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTracker: %v", err)
	}
	t.Cleanup(func() { tr.Close() })
	return tr, dir
}

// ─── CommitBatch ───────────────────────────────────────────────────────

func TestSQLiteTracker_CommitBatch_MultipleSnapshots(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snaps := []*models.Snapshot{
		{URL: "https://example.com/a", StatusCode: 200, Body: []byte("page-a"), Headers: map[string][]string{}},
		{URL: "https://example.com/b", StatusCode: 200, Body: []byte("page-b"), Headers: map[string][]string{}},
		{URL: "https://example.com/c", StatusCode: 200, Body: []byte("page-c"), Headers: map[string][]string{}},
	}

	cr, err := tr.CommitBatch(ctx, snaps, "batch commit", "test")
	if err != nil {
		t.Fatalf("CommitBatch: %v", err)
	}
	if cr == nil {
		t.Fatal("CommitBatch returned nil")
	}

	got, err := tr.GetSnapshots(ctx, cr.Version.ID)
	if err != nil {
		t.Fatalf("GetSnapshots: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(got))
	}
}

func TestSQLiteTracker_CommitBatch_Empty(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	_, err := tr.CommitBatch(ctx, []*models.Snapshot{}, "empty", "test")
	if err == nil {
		t.Fatal("expected error for empty batch")
	}
}

// ─── ReadHEAD / HEADExists ────────────────────────────────────────────

func TestSQLiteTracker_HEADExists_FalseInitially(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)

	exists, err := tr.HEADExists()
	if err != nil {
		t.Fatalf("HEADExists: %v", err)
	}
	if exists {
		t.Error("expected HEAD to not exist initially")
	}
}

func TestSQLiteTracker_ReadHEAD_AfterCommit(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snap := &models.Snapshot{URL: "https://example.com", Body: []byte("test"), StatusCode: 200}
	cr, err := tr.Commit(ctx, snap, "first", "test")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	exists, err := tr.HEADExists()
	if err != nil {
		t.Fatalf("HEADExists: %v", err)
	}
	if !exists {
		t.Error("expected HEAD to exist after commit")
	}

	head, err := tr.ReadHEAD()
	if err != nil {
		t.Fatalf("ReadHEAD: %v", err)
	}
	if head != cr.Version.ID {
		t.Errorf("expected HEAD=%q, got %q", cr.Version.ID, head)
	}
}

func TestSQLiteTracker_ReadHEAD_UpdatesOnSecondCommit(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snap1 := &models.Snapshot{URL: "https://example.com", Body: []byte("v1"), StatusCode: 200}
	_, err := tr.Commit(ctx, snap1, "first", "")
	if err != nil {
		t.Fatalf("first Commit: %v", err)
	}

	snap2 := &models.Snapshot{URL: "https://example.com", Body: []byte("v2"), StatusCode: 200}
	cr2, err := tr.Commit(ctx, snap2, "second", "")
	if err != nil {
		t.Fatalf("second Commit: %v", err)
	}

	head, err := tr.ReadHEAD()
	if err != nil {
		t.Fatalf("ReadHEAD: %v", err)
	}
	if head != cr2.Version.ID {
		t.Errorf("HEAD should point to latest commit, got %q", head)
	}
}

// ─── GetSnapshotByURL ──────────────────────────────────────────────────

func TestSQLiteTracker_GetSnapshotByURL(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	url := "https://example.com/specific"
	snap := &models.Snapshot{URL: url, Body: []byte("specific"), StatusCode: 200}
	_, err := tr.Commit(ctx, snap, "commit", "")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	got, err := tr.GetSnapshotByURL(ctx, url)
	if err != nil {
		t.Fatalf("GetSnapshotByURL: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if got.URL != url {
		t.Errorf("expected URL=%q, got %q", url, got.URL)
	}
}

// ─── GetParentVersionID ────────────────────────────────────────────────

func TestSQLiteTracker_GetParentVersionID(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snap1 := &models.Snapshot{URL: "https://example.com", Body: []byte("v1"), StatusCode: 200}
	cr1, err := tr.Commit(ctx, snap1, "first", "")
	if err != nil {
		t.Fatalf("first Commit: %v", err)
	}

	snap2 := &models.Snapshot{URL: "https://example.com", Body: []byte("v2"), StatusCode: 200}
	cr2, err := tr.Commit(ctx, snap2, "second", "")
	if err != nil {
		t.Fatalf("second Commit: %v", err)
	}

	parent, err := tr.GetParentVersionID(ctx, cr2.Version.ID)
	if err != nil {
		t.Fatalf("GetParentVersionID: %v", err)
	}
	if parent != cr1.Version.ID {
		t.Errorf("expected parent=%q, got %q", cr1.Version.ID, parent)
	}
}

func TestSQLiteTracker_GetParentVersionID_InitialCommitEmpty(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snap := &models.Snapshot{URL: "https://example.com", Body: []byte("v1"), StatusCode: 200}
	cr, err := tr.Commit(ctx, snap, "first", "")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	parent, err := tr.GetParentVersionID(ctx, cr.Version.ID)
	if err != nil {
		t.Fatalf("GetParentVersionID: %v", err)
	}
	if parent != "" {
		t.Errorf("expected empty parent for initial commit, got %q", parent)
	}
}

// ─── Checkout updates HEAD ─────────────────────────────────────────────

func TestSQLiteTracker_Checkout_UpdatesHEAD(t *testing.T) {
	t.Parallel()
	tr, dir := newTestTracker(t)
	ctx := context.Background()

	snap1 := &models.Snapshot{URL: "https://example.com", Body: []byte("v1"), StatusCode: 200}
	cr1, err := tr.Commit(ctx, snap1, "first", "")
	if err != nil {
		t.Fatalf("Commit 1: %v", err)
	}

	snap2 := &models.Snapshot{URL: "https://example.com", Body: []byte("v2"), StatusCode: 200}
	_, err = tr.Commit(ctx, snap2, "second", "")
	if err != nil {
		t.Fatalf("Commit 2: %v", err)
	}

	// Checkout first version
	if err := tr.Checkout(ctx, cr1.Version.ID); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	headPath := filepath.Join(dir, ".moku", "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("ReadFile HEAD: %v", err)
	}
	if string(content) != cr1.Version.ID {
		t.Errorf("HEAD should point to checked-out version, got %q", string(content))
	}
}

// ─── DiffSnapshots ─────────────────────────────────────────────────────

func TestSQLiteTracker_DiffSnapshots(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)
	ctx := context.Background()

	snap1 := &models.Snapshot{URL: "https://example.com", Body: []byte("before"), StatusCode: 200}
	cr1, err := tr.Commit(ctx, snap1, "v1", "")
	if err != nil {
		t.Fatalf("Commit 1: %v", err)
	}

	snap2 := &models.Snapshot{URL: "https://example.com", Body: []byte("after"), StatusCode: 200}
	cr2, err := tr.Commit(ctx, snap2, "v2", "")
	if err != nil {
		t.Fatalf("Commit 2: %v", err)
	}

	snaps1, _ := tr.GetSnapshots(ctx, cr1.Version.ID)
	snaps2, _ := tr.GetSnapshots(ctx, cr2.Version.ID)

	if len(snaps1) == 0 || len(snaps2) == 0 {
		t.Fatal("expected snapshots for both versions")
	}

	diff, err := tr.DiffSnapshots(ctx, snaps1[0].ID, snaps2[0].ID)
	if err != nil {
		t.Fatalf("DiffSnapshots: %v", err)
	}
	if diff == nil {
		t.Fatal("DiffSnapshots returned nil")
	}
	if len(diff.BodyDiff.Chunks) == 0 {
		t.Error("expected non-empty diff chunks between 'before' and 'after'")
	}
}

// ─── DB accessor ───────────────────────────────────────────────────────

func TestSQLiteTracker_DB_NotNil(t *testing.T) {
	t.Parallel()
	tr, _ := newTestTracker(t)

	if tr.DB() == nil {
		t.Error("expected non-nil DB()")
	}
}
