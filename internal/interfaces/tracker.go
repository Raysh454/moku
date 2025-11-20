package interfaces

import (
	"context"

	"github.com/raysh454/moku/internal/model"
)

// Tracker is the minimal cross-package contract for versioning website snapshots.
// Implementations should be safe for concurrent use.
type Tracker interface {
	// Commit stores a snapshot and returns a Version record representing the commit.
	// 'message' is a human message describing the change; author is optional.
	Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.Version, error)

	// Diff computes a delta between two versions identified by their IDs.
	// If baseID == "" treat it as an empty/base snapshot.
	Diff(ctx context.Context, baseID, headID string) (*model.DiffResult, error)

	// Get returns the snapshot for a specific version ID.
	Get(ctx context.Context, versionID string) (*model.Snapshot, error)

	// List returns recent versions (e.g., head-first). The semantics of pagination
	// can be added later.
	List(ctx context.Context, limit int) ([]*model.Version, error)

	// Close releases resources used by the tracker.
	Close() error
}
