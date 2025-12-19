package assessor

import "testing"

func TestDiffScores_BothNil(t *testing.T) {
	diff := DiffScores(nil, nil)

	if diff.ScoreBase != 0 || diff.ScoreHead != 0 || diff.ScoreDelta != 0 {
		t.Fatalf("expected zero scores for nil inputs, got base=%v head=%v delta=%v", diff.ScoreBase, diff.ScoreHead, diff.ScoreDelta)
	}
	if len(diff.FeatureDeltas) != 0 {
		t.Errorf("expected no feature deltas, got %v", diff.FeatureDeltas)
	}
	if len(diff.RuleDeltas) != 0 {
		t.Errorf("expected no rule deltas, got %v", diff.RuleDeltas)
	}
}

func TestDiffScores_ComputesDeltas(t *testing.T) {
	base := &ScoreResult{
		Score: 0.3,
		RawFeatures: map[string]float64{
			"a": 1,
			"b": 2,
		},
		ContribByRule: map[string]float64{
			"r1": 0.1,
			"r2": 0.2,
		},
	}
	head := &ScoreResult{
		Score: 0.6,
		RawFeatures: map[string]float64{
			"b": 3,
			"c": 4,
		},
		ContribByRule: map[string]float64{
			"r2": 0.1,
			"r3": 0.5,
		},
	}

	diff := DiffScores(base, head)

	if diff.ScoreBase != 0.3 || diff.ScoreHead != 0.6 || diff.ScoreDelta != 0.3 {
		t.Errorf("unexpected score values: base=%v head=%v delta=%v", diff.ScoreBase, diff.ScoreHead, diff.ScoreDelta)
	}

	if got := diff.FeatureDeltas["b"]; got != 1 {
		t.Errorf("expected feature delta for b == 1, got %v", got)
	}
	if got := diff.FeatureDeltas["c"]; got != 4 {
		t.Errorf("expected feature delta for c == 4, got %v", got)
	}
	if got := diff.FeatureDeltas["a"]; got != -1 {
		t.Errorf("expected feature delta for a == -1, got %v", got)
	}

	if got := diff.RuleDeltas["r2"]; got != -0.1 {
		t.Errorf("expected rule delta for r2 == -0.1, got %v", got)
	}
	if got := diff.RuleDeltas["r3"]; got != 0.5 {
		t.Errorf("expected rule delta for r3 == 0.5, got %v", got)
	}
	if got := diff.RuleDeltas["r1"]; got != -0.1 {
		t.Errorf("expected rule delta for r1 == -0.1, got %v", got)
	}
}
