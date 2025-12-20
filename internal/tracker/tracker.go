package tracker

import (
	"context"
	"database/sql"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/tracker/models"
)

// Tracker is the minimal cross-package contract for versioning website snapshots.
// Implementations should be safe for concurrent use.
type Tracker interface {
	// Commit stores a snapshot and returns a Version record representing the commit.
	// 'message' is a human message describing the change; author is optional.
	Commit(ctx context.Context, snapshot *models.Snapshot, message string, author string) (*models.CommitResult, error)

	// CommitBatch stores multiple snapshots and returns a single CommitResult containing all snapshots.
	CommitBatch(ctx context.Context, snapshots []*models.Snapshot, message, author string) (*models.CommitResult, error)

	// ScoreAndAttributeVersion assigns a score (security relavance) for a given commit result
	ScoreAndAttributeVersion(ctx context.Context, cr *models.CommitResult, scoreTimeout time.Duration) error

	// GetScoreResult retrieves the ScoreResult associated with a specific snapshot ID.
	GetScoreResultFromSnapshotID(ctx context.Context, snapshotID string) (*assessor.ScoreResult, error)

	// GetScoreResultsFromVersionID retrieves all ScoreResults associated with a specific version ID.
	GetScoreResultsFromVersionID(ctx context.Context, versionID string) ([]*assessor.ScoreResult, error)

	// GetSecurityDiffOverview computes a security-focused diff overview between two versions.
	// If baseID == "" treat it as an empty/base version.
	GetSecurityDiffOverview(ctx context.Context, baseID, headID string) (*assessor.SecurityDiffOverview, error)

	// GetSecurityDiff gets a detailed security diff between two snapshots.
	// Enforces that both snapshots belong to the same URL.
	GetSecurityDiff(ctx context.Context, baseSnapshotID, headSnapshotID string) (*assessor.SecurityDiff, error)

	// SetAssessor sets the Assessor used by ScoreAndAttributeVersion to produce a score.
	SetAssessor(a assessor.Assessor)

	// Diff computes the text delta between two versions identified by their IDs.
	// If baseID == "" treat it as an empty/base snapshot.
	Diff(ctx context.Context, baseID, headID string) (*models.CombinedMultiDiff, error)

	// GetSnapshots returns all snapshots for a specific version ID.
	// A version may reference multiple snapshots directly through the version_id foreign key.
	GetSnapshots(ctx context.Context, versionID string) ([]*models.Snapshot, error)

	// ListVersions returns recent versions (e.g., head-first). The semantics of pagination
	// can be added later.
	ListVersions(ctx context.Context, limit int) ([]*models.Version, error)

	// Checkout updates the working tree to match a specific version.
	// This restores all files from the specified version to the working directory.
	Checkout(ctx context.Context, versionID string) error

	// Returns a reference to the underlying database (Owned by Tracker)
	DB() *sql.DB

	// Close releases resources used by the tracker.
	Close() error
}
