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
	Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.CommitResult, error)

	// CommitBatch stores multiple snapshots and returns their corresponding Version records.
	CommitBatch(ctx context.Context, snapshots []*model.Snapshot, message, author string) ([]*model.CommitResult, error)

	// ScoreAndAttributeVersion assigns a score (security relavance) for a given commit result
	ScoreAndAttributeVersion(ctx context.Context, cr *model.CommitResult) error

	// SetAssessor sets the Assessor used by ScoreAndAttributeVersion to produce a score.
	SetAssessor(a Assessor)

	// Diff computes a delta between two versions identified by their IDs.
	// If baseID == "" treat it as an empty/base snapshot.
	Diff(ctx context.Context, baseID, headID string) (*model.DiffResult, error)

	// Get returns the snapshot for a specific version ID.
	Get(ctx context.Context, versionID string) (*model.Snapshot, error)

	// List returns recent versions (e.g., head-first). The semantics of pagination
	// can be added later.
	List(ctx context.Context, limit int) ([]*model.Version, error)

	// Checkout updates the working tree to match a specific version.
	// This restores all files from the specified version to the working directory.
	Checkout(ctx context.Context, versionID string) error

	// Close releases resources used by the tracker.
	Close() error
}
