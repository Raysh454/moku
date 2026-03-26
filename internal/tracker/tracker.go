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

	// BeginCommit starts a new multi-batch commit transaction.
	// It generates a version ID upfront and begins a database transaction.
	// Use this when you need to commit snapshots across multiple batches
	// while maintaining a single version ID.
	//
	// Example workflow:
	//   pc, err := tracker.BeginCommit(ctx, "Fetch 2500 pages", "fetcher")
	//   if err != nil { return err }
	//   defer tracker.CancelCommit(ctx, pc) // Cleanup on error
	//
	//   for batch := range batches {
	//       if err := tracker.AddSnapshots(ctx, pc, batch); err != nil {
	//           return err // CancelCommit called by defer
	//       }
	//   }
	//
	//   result, err := tracker.FinalizeCommit(ctx, pc)
	//
	// Returns a PendingCommit handle that must be passed to AddSnapshots,
	// FinalizeCommit, or CancelCommit.
	BeginCommit(ctx context.Context, message, author string) (*models.PendingCommit, error)

	// AddSnapshots adds a batch of snapshots to a pending commit.
	// All snapshots will be associated with the same version ID from BeginCommit.
	// This method can be called multiple times for a single PendingCommit.
	//
	// The transaction remains open; call FinalizeCommit to complete the commit
	// or CancelCommit to rollback.
	//
	// Returns an error if the PendingCommit is invalid or the transaction failed.
	AddSnapshots(ctx context.Context, pc *models.PendingCommit, snapshots []*models.Snapshot) error

	// FinalizeCommit completes a pending commit by computing diffs, committing
	// the transaction, and updating HEAD.
	//
	// After FinalizeCommit succeeds, the PendingCommit is no longer valid and
	// should not be used with AddSnapshots or CancelCommit.
	//
	// Returns a CommitResult containing the version, all snapshots added via
	// AddSnapshots, and computed diffs.
	FinalizeCommit(ctx context.Context, pc *models.PendingCommit) (*models.CommitResult, error)

	// CancelCommit rolls back a pending commit and cleans up resources.
	// This should be called if AddSnapshots or FinalizeCommit fails, or if
	// the operation is cancelled.
	//
	// It's safe to call CancelCommit multiple times or on an already-finalized commit.
	// Best practice is to defer CancelCommit immediately after BeginCommit.
	CancelCommit(ctx context.Context, pc *models.PendingCommit) error

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
	DiffVersions(ctx context.Context, baseID, headID string) (*models.CombinedMultiDiff, error)

	// DiffSnapshots computes the text delta between two snapshots identified by their IDs.
	DiffSnapshots(ctx context.Context, baseSnapshotID, headSnapshotID string) (*models.CombinedFileDiff, error)

	// GetSnapshot retrieves a snapshot by its ID.
	GetSnapshot(ctx context.Context, snapshotID string) (*models.Snapshot, error)

	// GetSnapshots returns all snapshots for a specific version ID.
	// A version may reference multiple snapshots directly through the version_id foreign key.
	GetSnapshots(ctx context.Context, versionID string) ([]*models.Snapshot, error)

	// GetSnapshotByURL retrieves the latest snapshot for a given URL.
	GetSnapshotByURL(ctx context.Context, url string) (*models.Snapshot, error)

	// GetSnapshotByURLAndVersionID retrieves a snapshot for a given URL and version ID.
	GetSnapshotByURLAndVersionID(ctx context.Context, url, versionID string) (*models.Snapshot, error)

	// GetParentVersionID returns the parent version ID of a given version.
	// If the version has no parent (e.g., initial commit), returns an empty string.
	GetParentVersionID(ctx context.Context, versionID string) (string, error)

	// ListVersions returns recent versions (e.g., head-first). The semantics of pagination
	// can be added later.
	ListVersions(ctx context.Context, limit int) ([]*models.Version, error)

	// Checkout updates the working tree to match a specific version.
	// This restores all files from the specified version to the working directory.
	Checkout(ctx context.Context, versionID string) error

	// HEADExists checks if a HEAD exists.
	HEADExists() (bool, error)

	// ReadHEAD returns the current head version ID.
	ReadHEAD() (string, error)

	// Returns a reference to the underlying database (Owned by Tracker)
	DB() *sql.DB

	// Close releases resources used by the tracker.
	Close() error
}
