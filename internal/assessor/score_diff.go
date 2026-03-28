package assessor

func DiffScores(base, head *ScoreResult) *ScoreDiff {
	if base == nil && head == nil {
		return &ScoreDiff{
			ScoreBase:     0,
			ScoreHead:     0,
			ScoreDelta:    0,
			FeatureDeltas: map[string]float64{},
			RuleDeltas:    map[string]float64{},
		}
	}
	if base == nil {
		base = &ScoreResult{
			Score:         0,
			RawFeatures:   map[string]float64{},
			ContribByRule: map[string]float64{},
		}
	}
	if head == nil {
		head = &ScoreResult{
			Score:         0,
			RawFeatures:   map[string]float64{},
			ContribByRule: map[string]float64{},
		}
	}

	// Scores
	diff := &ScoreDiff{
		ScoreBase:     base.Score,
		ScoreHead:     head.Score,
		ScoreDelta:    head.Score - base.Score,
		FeatureDeltas: make(map[string]float64),
		RuleDeltas:    make(map[string]float64),
	}

	// Feature deltas: head.RawFeatures[k] - base.RawFeatures[k]
	for k, hv := range head.RawFeatures {
		bv := base.RawFeatures[k]
		delta := hv - bv
		if delta != 0 {
			diff.FeatureDeltas[k] = delta
		}
	}
	// Also include features that existed only in base
	for k, bv := range base.RawFeatures {
		if _, ok := head.RawFeatures[k]; !ok && bv != 0 {
			diff.FeatureDeltas[k] = -bv
		}
	}

	// Rule/feature contribution deltas: head.ContribByRule[k] - base.ContribByRule[k]
	for k, hv := range head.ContribByRule {
		bv := base.ContribByRule[k]
		delta := hv - bv
		if delta != 0 {
			diff.RuleDeltas[k] = delta
		}
	}
	for k, bv := range base.ContribByRule {
		if _, ok := head.ContribByRule[k]; !ok && bv != 0 {
			diff.RuleDeltas[k] = -bv
		}
	}

	return diff
}
