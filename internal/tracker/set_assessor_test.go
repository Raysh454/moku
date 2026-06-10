package tracker_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

// TestSetAssessor_AfterConstruction_PersistsScores: given a tracker
// constructed WITHOUT an assessor, when SetAssessor injects one later and a
// committed version is scored, then score results are persisted. The
// injected assessor must reach the score-persistence layer — passing the
// tracker's own nil-check while the score tracker still holds the nil
// captured at construction silently skips persistence.
func TestSetAssessor_AfterConstruction_PersistsScores(t *testing.T) {
	t.Parallel()

	// Arrange: no assessor at construction time.
	var full tracker.Tracker = newRoleTestTracker(t, nil)
	ctx := context.Background()
	committed, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com/settings",
		Body: []byte("<html><body>settings page</body></html>"),
	})
	if err != nil {
		t.Fatalf("arrange commit failed: %v", err)
	}

	// Act: inject the assessor after construction, then score the version.
	full.SetAssessor(&stubAssessor{})
	results, err := scoreVersion(ctx, full, committed)

	// Assert: the late-bound assessor's scores must be persisted.
	if err != nil {
		t.Fatalf("scoring after SetAssessor returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 persisted score result after SetAssessor, got %d", len(results))
	}
	if results[0].Score != stubAssessorScore {
		t.Errorf("expected score %v, got %v", stubAssessorScore, results[0].Score)
	}
}
