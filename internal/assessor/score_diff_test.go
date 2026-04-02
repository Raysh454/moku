package assessor

import (
	"math"
	"testing"
)

const floatEpsilon = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < floatEpsilon
}

func TestDiffScores_BothNil(t *testing.T) {
	diff := DiffScores(nil, nil)

	if diff.ScoreBase != 0 || diff.ScoreHead != 0 || diff.ScoreDelta != 0 {
		t.Fatalf("expected zero scores for nil inputs, got base=%v head=%v delta=%v", diff.ScoreBase, diff.ScoreHead, diff.ScoreDelta)
	}
	if diff.ExposureDelta != 0 || diff.HardeningDelta != 0 || diff.ChangeScoreDelta != 0 {
		t.Errorf("expected zero deltas for nil inputs")
	}
}

func TestDiffScores_ComputesDeltas(t *testing.T) {
	base := &ScoreResult{
		Score:          0.3,
		ExposureScore:  0.5,
		HardeningScore: 0.4,
		ChangeScore:    0.1,
	}
	head := &ScoreResult{
		Score:          0.6,
		ExposureScore:  0.8,
		HardeningScore: 0.2,
		ChangeScore:    0.3,
	}

	diff := DiffScores(base, head)

	if diff.ScoreBase != 0.3 || diff.ScoreHead != 0.6 || diff.ScoreDelta != 0.3 {
		t.Errorf("unexpected score values: base=%v head=%v delta=%v", diff.ScoreBase, diff.ScoreHead, diff.ScoreDelta)
	}

	if !approxEqual(diff.ExposureDelta, 0.3) {
		t.Errorf("expected ExposureDelta ~0.3, got %v", diff.ExposureDelta)
	}

	if !approxEqual(diff.HardeningDelta, -0.2) {
		t.Errorf("expected HardeningDelta ~-0.2, got %v", diff.HardeningDelta)
	}

	if !approxEqual(diff.ChangeScoreDelta, 0.2) {
		t.Errorf("expected ChangeScoreDelta ~0.2, got %v", diff.ChangeScoreDelta)
	}
}
