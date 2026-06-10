package tracker_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/tracker/models"
)

const (
	persistedScoreValue        = 0.5
	persistedContribution      = 0.3
	persistedEvidenceKey       = "login-form"
	persistedEvidenceSeverity  = "high"
	persistedEvidenceRuleID    = "forms:login"
	persistedEvidenceDescr     = "exposed login form"
	persistedScoreResultsCount = 1
)

// TestPersistScore_StoresPrecomputedResult: given a tracker constructed WITHOUT
// an assessor and a committed snapshot, when a precomputed ScoreResult is
// handed to PersistScore, then reading the version's scores back returns that
// result with its evidence intact. PersistScore is persistence-only: producing
// the score is the caller's responsibility, so no assessor is required.
func TestPersistScore_StoresPrecomputedResult(t *testing.T) {
	t.Parallel()

	// Arrange: a tracker with no assessor, plus a committed snapshot to attribute.
	var full = newRoleTestTracker(t)
	ctx := context.Background()
	committed, err := commitSnapshot(ctx, full, &models.Snapshot{
		URL:  "https://example.com/login",
		Body: []byte("<html><body><form id=\"login\"></form></body></html>"),
	})
	if err != nil {
		t.Fatalf("arrange commit failed: %v", err)
	}

	snapshot := committed.Snapshots[0]
	result := &assessor.ScoreResult{
		Score:      persistedScoreValue,
		SnapshotID: snapshot.ID,
		VersionID:  committed.Version.ID,
		Evidence: []assessor.EvidenceItem{
			{
				Key:          persistedEvidenceKey,
				RuleID:       persistedEvidenceRuleID,
				Severity:     persistedEvidenceSeverity,
				Description:  persistedEvidenceDescr,
				Contribution: persistedContribution,
			},
		},
	}

	// Act: persist the precomputed result, then read it back.
	if err := full.PersistScore(ctx, result, snapshot.ID, committed.Version.ID, snapshot.URL); err != nil {
		t.Fatalf("PersistScore returned error: %v", err)
	}
	results, err := full.GetScoreResultsFromVersionID(ctx, committed.Version.ID)
	if err != nil {
		t.Fatalf("GetScoreResultsFromVersionID returned error: %v", err)
	}

	// Assert: the persisted result round-trips, evidence and all.
	if len(results) != persistedScoreResultsCount {
		t.Fatalf("expected %d persisted score result, got %d", persistedScoreResultsCount, len(results))
	}
	got := results[0]
	if got.Score != persistedScoreValue {
		t.Errorf("expected score %v, got %v", persistedScoreValue, got.Score)
	}
	if len(got.Evidence) != 1 {
		t.Fatalf("expected 1 persisted evidence item, got %d", len(got.Evidence))
	}
	evidence := got.Evidence[0]
	if evidence.Key != persistedEvidenceKey {
		t.Errorf("expected evidence key %q, got %q", persistedEvidenceKey, evidence.Key)
	}
	if evidence.Contribution != persistedContribution {
		t.Errorf("expected evidence contribution %v, got %v", persistedContribution, evidence.Contribution)
	}
}
