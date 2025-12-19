package score

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor"
)

func TestDiffScoreResults_EmptyResults(t *testing.T) {
	base := &assessor.ScoreResult{
		Score:         0.0,
		ContribByRule: map[string]float64{},
	}
	head := &assessor.ScoreResult{
		Score:         0.0,
		ContribByRule: map[string]float64{},
	}

	delta := DiffScoreResults(base, head, "https://example.com", "v1", "v2")

	if delta == nil {
		t.Fatal("Expected non-nil delta")
	}
	if delta.URL != "https://example.com" {
		t.Errorf("Expected URL to be https://example.com, got %s", delta.URL)
	}
	if delta.BaseVersion != "v1" {
		t.Errorf("Expected BaseVersion to be v1, got %s", delta.BaseVersion)
	}
	if delta.HeadVersion != "v2" {
		t.Errorf("Expected HeadVersion to be v2, got %s", delta.HeadVersion)
	}
	if delta.Delta != 0.0 {
		t.Errorf("Expected Delta to be 0.0, got %f", delta.Delta)
	}
	if len(delta.RuleDeltas) != 0 {
		t.Errorf("Expected 0 RuleDeltas, got %d", len(delta.RuleDeltas))
	}
}

func TestDiffScoreResults_AddedRules(t *testing.T) {
	base := &assessor.ScoreResult{
		Score:         0.1,
		ContribByRule: map[string]float64{},
		MatchedRules:  []assessor.Rule{},
	}
	head := &assessor.ScoreResult{
		Score: 0.3,
		ContribByRule: map[string]float64{
			"rule1": 0.1,
			"rule2": 0.2,
		},
		MatchedRules: []assessor.Rule{
			{ID: "rule1", Key: "test-rule-1", Severity: "medium", Weight: 0.1},
			{ID: "rule2", Key: "test-rule-2", Severity: "high", Weight: 0.2},
		},
	}

	delta := DiffScoreResults(base, head, "https://example.com", "v1", "v2")

	if delta.BaseScore != 0.1 {
		t.Errorf("Expected BaseScore to be 0.1, got %f", delta.BaseScore)
	}
	if delta.HeadScore != 0.3 {
		t.Errorf("Expected HeadScore to be 0.3, got %f", delta.HeadScore)
	}
	// Use tolerance for floating point comparison
	expectedDelta := 0.2
	if delta.Delta < expectedDelta-0.0001 || delta.Delta > expectedDelta+0.0001 {
		t.Errorf("Expected Delta to be around 0.2, got %f", delta.Delta)
	}
	if len(delta.RuleDeltas) != 2 {
		t.Fatalf("Expected 2 RuleDeltas, got %d", len(delta.RuleDeltas))
	}

	// Should be sorted by absolute delta descending
	if delta.RuleDeltas[0].RuleID != "rule2" {
		t.Errorf("Expected first RuleDelta to be rule2, got %s", delta.RuleDeltas[0].RuleID)
	}
	if delta.RuleDeltas[0].Delta != 0.2 {
		t.Errorf("Expected first RuleDelta.Delta to be 0.2, got %f", delta.RuleDeltas[0].Delta)
	}
	if delta.RuleDeltas[0].Severity != "high" {
		t.Errorf("Expected first RuleDelta.Severity to be high, got %s", delta.RuleDeltas[0].Severity)
	}
}

func TestDiffScoreResults_RemovedRules(t *testing.T) {
	base := &assessor.ScoreResult{
		Score: 0.5,
		ContribByRule: map[string]float64{
			"rule1": 0.3,
			"rule2": 0.2,
		},
		MatchedRules: []assessor.Rule{
			{ID: "rule1", Key: "test-rule-1", Severity: "high", Weight: 0.3},
			{ID: "rule2", Key: "test-rule-2", Severity: "medium", Weight: 0.2},
		},
	}
	head := &assessor.ScoreResult{
		Score:         0.0,
		ContribByRule: map[string]float64{},
		MatchedRules:  []assessor.Rule{},
	}

	delta := DiffScoreResults(base, head, "https://example.com", "v1", "v2")

	if delta.Delta != -0.5 {
		t.Errorf("Expected Delta to be -0.5, got %f", delta.Delta)
	}
	if len(delta.RuleDeltas) != 2 {
		t.Fatalf("Expected 2 RuleDeltas, got %d", len(delta.RuleDeltas))
	}

	// Should be sorted by absolute delta descending
	if delta.RuleDeltas[0].Delta >= 0 {
		t.Errorf("Expected negative delta for removed rule")
	}
}

func TestDiffScoreResults_ChangedRules(t *testing.T) {
	base := &assessor.ScoreResult{
		Score: 0.3,
		ContribByRule: map[string]float64{
			"rule1": 0.1,
			"rule2": 0.2,
		},
		MatchedRules: []assessor.Rule{
			{ID: "rule1", Key: "test-rule-1", Severity: "low", Weight: 0.1},
			{ID: "rule2", Key: "test-rule-2", Severity: "medium", Weight: 0.2},
		},
	}
	head := &assessor.ScoreResult{
		Score: 0.6,
		ContribByRule: map[string]float64{
			"rule1": 0.4,
			"rule2": 0.2,
		},
		MatchedRules: []assessor.Rule{
			{ID: "rule1", Key: "test-rule-1", Severity: "low", Weight: 0.4},
			{ID: "rule2", Key: "test-rule-2", Severity: "medium", Weight: 0.2},
		},
	}

	delta := DiffScoreResults(base, head, "https://example.com", "v1", "v2")

	// Use tolerance for floating point comparison
	expectedDelta := 0.3
	if delta.Delta < expectedDelta-0.0001 || delta.Delta > expectedDelta+0.0001 {
		t.Errorf("Expected Delta to be around 0.3, got %f", delta.Delta)
	}
	if len(delta.RuleDeltas) != 2 {
		t.Fatalf("Expected 2 RuleDeltas, got %d", len(delta.RuleDeltas))
	}

	// rule1 changed from 0.1 to 0.4, delta = 0.3
	// rule2 unchanged, delta = 0.0
	// Should be sorted by absolute delta descending
	if delta.RuleDeltas[0].RuleID != "rule1" {
		t.Errorf("Expected first RuleDelta to be rule1, got %s", delta.RuleDeltas[0].RuleID)
	}
	if delta.RuleDeltas[0].Base != 0.1 {
		t.Errorf("Expected rule1 Base to be 0.1, got %f", delta.RuleDeltas[0].Base)
	}
	if delta.RuleDeltas[0].Head != 0.4 {
		t.Errorf("Expected rule1 Head to be 0.4, got %f", delta.RuleDeltas[0].Head)
	}
	// Use tolerance for floating point comparison
	expectedRuleDelta := 0.3
	if delta.RuleDeltas[0].Delta < expectedRuleDelta-0.0001 || delta.RuleDeltas[0].Delta > expectedRuleDelta+0.0001 {
		t.Errorf("Expected rule1 Delta to be around 0.3, got %f", delta.RuleDeltas[0].Delta)
	}
}

func TestDiffScoreResults_NilResults(t *testing.T) {
	delta := DiffScoreResults(nil, nil, "https://example.com", "v1", "v2")

	if delta.BaseScore != 0.0 {
		t.Errorf("Expected BaseScore to be 0.0 for nil base, got %f", delta.BaseScore)
	}
	if delta.HeadScore != 0.0 {
		t.Errorf("Expected HeadScore to be 0.0 for nil head, got %f", delta.HeadScore)
	}
	if delta.Delta != 0.0 {
		t.Errorf("Expected Delta to be 0.0, got %f", delta.Delta)
	}
}
