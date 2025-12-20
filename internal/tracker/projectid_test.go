package tracker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/logging"
)

// TestSetAndGetProjectID verifies basic insert and read behavior and idempotency.
func TestSetAndGetProjectID(t *testing.T) {
	t.Parallel()

	siteDir := t.TempDir()
	logger := logging.NewStdoutLogger("tracker-test")

	// create tracker without a preconfigured project id but with existing storage path
	if err := os.MkdirAll(filepath.Join(siteDir, ".moku"), 0o755); err != nil {
		t.Fatalf("failed to create .moku dir: %v", err)
	}
	tr, err := NewSQLiteTracker(&Config{StoragePath: siteDir}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTrackerWithConfig failed: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// set project id
	if err := tr.SetProjectID(ctx, "proj-123", false); err != nil {
		t.Fatalf("SetProjectID failed: %v", err)
	}

	// get project id
	pid, err := tr.GetProjectID(ctx)
	if err != nil {
		t.Fatalf("GetProjectID failed: %v", err)
	}
	if pid != "proj-123" {
		t.Fatalf("expected project id 'proj-123', got %q", pid)
	}

	// idempotent: set same value again should succeed
	if err := tr.SetProjectID(ctx, "proj-123", false); err != nil {
		t.Fatalf("SetProjectID (idempotent) failed: %v", err)
	}
}

// TestSetProjectID_MismatchAndForce verifies that attempting to set a different project id
// without force returns ErrProjectIDMismatch, and that force overwrites.
func TestSetProjectID_MismatchAndForce(t *testing.T) {
	t.Parallel()

	siteDir := t.TempDir()
	logger := logging.NewStdoutLogger("tracker-test")

	if err := os.MkdirAll(filepath.Join(siteDir, ".moku"), 0o755); err != nil {
		t.Fatalf("failed to create .moku dir: %v", err)
	}
	tr, err := NewSQLiteTracker(&Config{StoragePath: siteDir}, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTrackerWithConfig failed: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()

	// initial set
	if err := tr.SetProjectID(ctx, "orig-id", false); err != nil {
		t.Fatalf("initial SetProjectID failed: %v", err)
	}

	// attempt to overwrite without force -> expect ErrProjectIDMismatch
	if err := tr.SetProjectID(ctx, "other-id", false); err == nil {
		t.Fatalf("expected ErrProjectIDMismatch, got nil")
	} else if err != ErrProjectIDMismatch {
		t.Fatalf("expected ErrProjectIDMismatch, got: %v", err)
	}

	// ensure value still original
	pid, err := tr.GetProjectID(ctx)
	if err != nil {
		t.Fatalf("GetProjectID failed: %v", err)
	}
	if pid != "orig-id" {
		t.Fatalf("expected project id 'orig-id' after failed overwrite, got %q", pid)
	}

	// force overwrite -> should succeed
	if err := tr.SetProjectID(ctx, "other-id", true); err != nil {
		t.Fatalf("SetProjectID with force failed: %v", err)
	}
	pid, err = tr.GetProjectID(ctx)
	if err != nil {
		t.Fatalf("GetProjectID failed: %v", err)
	}
	if pid != "other-id" {
		t.Fatalf("expected project id 'other-id' after forced overwrite, got %q", pid)
	}
}

// TestNewSQLiteTrackerWithConfig_ProjectID demonstrates that providing ProjectID in Config
// during tracker construction will populate meta. It also verifies that a conflicting
// ProjectID during construction fails unless ForceProjectID is set.
func TestNewSQLiteTrackerWithConfig_ProjectID(t *testing.T) {
	t.Parallel()

	siteDir := t.TempDir()
	logger := logging.NewStdoutLogger("tracker-test")

	// create tracker A with project id "p-a"
	cfgA := &Config{ProjectID: "p-a", StoragePath: siteDir}
	trA, err := NewSQLiteTracker(cfgA, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTrackerWithConfig (A) failed: %v", err)
	}
	// close to release DB file for subsequent open
	trA.Close()

	// attempt to create tracker B with different project id -> should fail
	cfgB := &Config{ProjectID: "p-b", ForceProjectID: false, StoragePath: siteDir}
	_, err = NewSQLiteTracker(cfgB, logger, nil)
	if err == nil {
		t.Fatalf("expected NewSQLiteTrackerWithConfig to fail on project id mismatch, but it succeeded")
	}

	// create tracker C with force overwrite -> should succeed and set project id to p-b
	cfgC := &Config{ProjectID: "p-b", ForceProjectID: true, StoragePath: siteDir}
	trC, err := NewSQLiteTracker(cfgC, logger, nil)
	if err != nil {
		t.Fatalf("NewSQLiteTrackerWithConfig (C) failed with force: %v", err)
	}
	defer trC.Close()

	pid, err := trC.GetProjectID(context.Background())
	if err != nil {
		t.Fatalf("GetProjectID failed: %v", err)
	}
	if pid != "p-b" {
		t.Fatalf("expected project id 'p-b' after forced creation, got %q", pid)
	}
}
