package assessor

const totalKnownFeatures = 44.0

func ComputeConfidence(features map[string]float64, ruleContribs map[string]float64, bodyLen int, statusCode int) float64 {
	var score float64

	nonZeroFeats := 0
	for _, v := range features {
		if v != 0 {
			nonZeroFeats++
		}
	}
	featureCoverage := float64(nonZeroFeats) / totalKnownFeatures
	score += featureCoverage * 0.4

	if len(ruleContribs) > 0 {
		score += 0.2
	}

	switch {
	case bodyLen > 1000:
		score += 0.2
	case bodyLen > 100:
		score += 0.1
	}

	if statusCode >= 200 && statusCode < 400 {
		score += 0.2
	} else if statusCode >= 400 {
		score += 0.05
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}
