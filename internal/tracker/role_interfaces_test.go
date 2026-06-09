package tracker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

// These tests prove the interface-segregated tracker contract from a
// consumer's point of view: each test hands the full Tracker to a function
// that depends on exactly one role interface, then verifies the role behaves
// against a real SQLiteTracker rooted in a per-test temp directory.

const (
	roleTestProjectID = "role-interface-test"
	roleCommitMessage = "role-interface commit"
	roleCommitAuthor  = "roles@example.com"
	historyListLimit  = 10
	stubAssessorScore = 42.5
	scoreWaitTimeout  = 5 * time.Second
)

// TestCommitStore_PersistsSnapshotAsNewVersion: given a consumer that depends
// only on the CommitStore role, when it commits a snapshot, then a new
// version is recorded with the supplied message and author.
func TestCommitStore_PersistsSnapshotAsNewVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	var full tracker.Tracker = newRoleTestTracker(t, nil)
	snapshot := &models.Snapshot{
		URL:  "https://example.com/login",
		Body: []byte("<html><body>login form</body></html>"),
	}

	// Act
	committed, err := commitSnapshot(context.Background(), full, snapshot)

	// Assert
	if err != nil {
		t.Fatalf("commit through CommitStore returned error: %v", err)
	}
	if committed == nil || committed.Version.ID == "" {
		t.Fatal("expected a commit result with a non-empty version ID")
	}
	if committed.Version.Message != roleCommitMessage {
		t.Errorf("expected message %q, got %q", roleCommitMessage, committed.Version.Message)
	}
	if committed.Version.Author != roleCommitAuthor {
		t.Errorf("expected author %q, got %q", roleCommitAuthor, committed.Version.Author)
	}
}

// commitSnapshot is a consumer that needs nothing beyond the CommitStore role.
func commitSnapshot(ctx context.Context, commits tracker.CommitStore, snapshot *models.Snapshot) (*models.CommitResult, error) {
	return commits.Commit(ctx, snapshot, roleCommitMessage, roleCommitAuthor)
}

// TestSnapshotReader_ReadsBackCommittedSnapshotBody: given a committed
// snapshot, when a consumer that depends only on the SnapshotReader role
// fetches the version's snapshots, then the stored body round-trips unchanged.
func TestSnapshotReader_ReadsBackCommittedSnapshotBody(t *testing.T) {
	t.Parallel()

	// Arrange
	var full tracker.Tracker = newRoleTestTracker(t, nil)
	ctx := context.Background()
	body := "<html><body>profile page</body></html>"
	committed, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com/profile",
		Body: []byte(body),
	})
	if err != nil {
		t.Fatalf("arrange commit failed: %v", err)
	}

	// Act
	snapshots, err := readVersionSnapshots(ctx, full, committed.Version.ID)

	// Assert
	if err != nil {
		t.Fatalf("read through SnapshotReader returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if got := string(snapshots[0].Body); got != body {
		t.Errorf("expected body %q, got %q", body, got)
	}
}

// readVersionSnapshots is a consumer that needs nothing beyond the SnapshotReader role.
func readVersionSnapshots(ctx context.Context, reader tracker.SnapshotReader, versionID string) ([]*models.Snapshot, error) {
	return reader.GetSnapshots(ctx, versionID)
}

// TestVersionHistory_NavigatesHEADAndParentChain: given two committed
// versions, when a consumer that depends only on the VersionHistory role
// inspects HEAD, the parent chain, and the version list, then it sees the
// second commit at HEAD with the first commit as its parent.
func TestVersionHistory_NavigatesHEADAndParentChain(t *testing.T) {
	t.Parallel()

	// Arrange
	var full tracker.Tracker = newRoleTestTracker(t, nil)
	ctx := context.Background()
	first, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>v1</body></html>"),
	})
	if err != nil {
		t.Fatalf("arrange first commit failed: %v", err)
	}
	second, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com",
		Body: []byte("<html><body>v2</body></html>"),
	})
	if err != nil {
		t.Fatalf("arrange second commit failed: %v", err)
	}

	// Act
	head, parentOfHead, versions, err := describeHistory(ctx, full)

	// Assert
	if err != nil {
		t.Fatalf("navigation through VersionHistory returned error: %v", err)
	}
	if head != second.Version.ID {
		t.Errorf("expected HEAD %q, got %q", second.Version.ID, head)
	}
	if parentOfHead != first.Version.ID {
		t.Errorf("expected parent of HEAD %q, got %q", first.Version.ID, parentOfHead)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 listed versions, got %d", len(versions))
	}
}

// describeHistory is a consumer that needs nothing beyond the VersionHistory role.
func describeHistory(ctx context.Context, history tracker.VersionHistory) (head, parentOfHead string, versions []*models.Version, err error) {
	exists, err := history.HEADExists()
	if err != nil {
		return "", "", nil, err
	}
	if !exists {
		return "", "", nil, errors.New("expected HEAD to exist after committing")
	}
	head, err = history.ReadHEAD()
	if err != nil {
		return "", "", nil, err
	}
	parentOfHead, err = history.GetParentVersionID(ctx, head)
	if err != nil {
		return "", "", nil, err
	}
	versions, err = history.ListVersions(ctx, historyListLimit)
	return head, parentOfHead, versions, err
}

// TestScoreStore_AttributesScoresToCommittedVersion: given a tracker whose
// assessor is a stub returning a fixed score, when a consumer that depends
// only on the ScoreStore role scores a commit and reads the results back,
// then exactly that score is attributed to the version.
func TestScoreStore_AttributesScoresToCommittedVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	var full tracker.Tracker = newRoleTestTracker(t, &stubAssessor{})
	ctx := context.Background()
	committed, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com/admin",
		Body: []byte("<html><body>admin panel</body></html>"),
	})
	if err != nil {
		t.Fatalf("arrange commit failed: %v", err)
	}

	// Act
	results, err := scoreVersion(ctx, full, committed)

	// Assert
	if err != nil {
		t.Fatalf("scoring through ScoreStore returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 score result, got %d", len(results))
	}
	if results[0].Score != stubAssessorScore {
		t.Errorf("expected score %v, got %v", stubAssessorScore, results[0].Score)
	}
}

// scoreVersion is a consumer that needs nothing beyond the ScoreStore role.
func scoreVersion(ctx context.Context, scores tracker.ScoreStore, committed *models.CommitResult) ([]*assessor.ScoreResult, error) {
	if err := scores.ScoreAndAttributeVersion(ctx, committed, scoreWaitTimeout); err != nil {
		return nil, err
	}
	return scores.GetScoreResultsFromVersionID(ctx, committed.Version.ID)
}

// TestTrackerAdmin_ExposesDatabaseAndReleasesResources: given a consumer that
// depends only on the TrackerAdmin role, when it inspects the owned database
// handle and closes the tracker, then both operations succeed.
func TestTrackerAdmin_ExposesDatabaseAndReleasesResources(t *testing.T) {
	t.Parallel()

	// Arrange
	var full tracker.Tracker = newRoleTestTracker(t, nil)

	// Act
	err := verifyDatabaseThenClose(full)

	// Assert
	if err != nil {
		t.Fatalf("TrackerAdmin consumer returned error: %v", err)
	}
}

// verifyDatabaseThenClose is a consumer that needs nothing beyond the TrackerAdmin role.
func verifyDatabaseThenClose(admin tracker.TrackerAdmin) error {
	if admin.DB() == nil {
		return errors.New("expected the tracker-owned database handle, got nil")
	}
	return admin.Close()
}

// newRoleTestTracker builds a real SQLiteTracker rooted in a per-test temp
// directory and ensures it is closed when the test finishes.
func newRoleTestTracker(t *testing.T, a assessor.Assessor) *tracker.SQLiteTracker {
	t.Helper()

	tr, err := tracker.NewSQLiteTracker(
		&tracker.Config{StoragePath: t.TempDir(), ProjectID: roleTestProjectID},
		logging.NewStdoutLogger("tracker-role-test"),
		a,
	)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}

// stubAssessor is an assessor.Assessor test double that returns a fixed score
// so attribution can be observed without real heuristics.
type stubAssessor struct{}

func (s *stubAssessor) ScoreSnapshot(_ context.Context, snapshot *models.Snapshot, versionID string) (*assessor.ScoreResult, error) {
	return &assessor.ScoreResult{
		Score:      stubAssessorScore,
		SnapshotID: snapshot.ID,
		VersionID:  versionID,
	}, nil
}

func (s *stubAssessor) Close() error { return nil }
