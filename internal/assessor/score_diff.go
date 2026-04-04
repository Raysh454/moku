package assessor

func DiffScores(base, head *ScoreResult) *ScoreDiff {
	if base == nil && head == nil {
		return &ScoreDiff{}
	}
	if base == nil {
		base = &ScoreResult{}
	}
	if head == nil {
		head = &ScoreResult{}
	}

	return &ScoreDiff{
		ScoreBase:      base.Score,
		ScoreHead:      head.Score,
		ScoreDelta:     head.Score - base.Score,
		ExposureDelta:  head.ExposureScore - base.ExposureScore,
		HardeningDelta: head.HardeningScore - base.HardeningScore,
	}
}
