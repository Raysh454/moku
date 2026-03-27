package assessor

import "math"

type CategoryScorer struct {
	categories map[string]Category
	weights    map[string]float64
	caps       map[string]float64
	maxContrib map[Category]float64
}

func NewCategoryScorer(categories map[string]Category, weights map[string]float64, caps map[string]float64) *CategoryScorer {
	cs := &CategoryScorer{
		categories: categories,
		weights:    weights,
		caps:       caps,
		maxContrib: make(map[Category]float64),
	}
	cs.computeMaxContribs()
	return cs
}

func (cs *CategoryScorer) computeMaxContribs() {
	for featureID, cat := range cs.categories {
		w := cs.weights[featureID]
		if w <= 0 {
			continue
		}
		maxVal := 1.0
		if cap, ok := cs.caps[featureID]; ok && cap > 0 {
			maxVal = cap
		}
		cs.maxContrib[cat] += w * maxVal
	}
}

func (cs *CategoryScorer) MaxContrib(cat Category) float64 {
	return cs.maxContrib[cat]
}

func (cs *CategoryScorer) ScoreCategory(cat Category, contribs map[string]float64) float64 {
	maxC := cs.maxContrib[cat]
	if maxC <= 0 {
		return 0.0
	}

	var raw float64
	for featureID, contrib := range contribs {
		if cs.categories[featureID] != cat {
			continue
		}
		capped := cs.capContrib(featureID, contrib)
		raw += capped
	}

	score := raw / maxC
	return math.Min(math.Max(score, 0.0), 1.0)
}

func (cs *CategoryScorer) RawContrib(cat Category, contribs map[string]float64) float64 {
	var raw float64
	for featureID, contrib := range contribs {
		if cs.categories[featureID] != cat {
			continue
		}
		raw += contrib
	}
	return raw
}

func (cs *CategoryScorer) ScoreAll(contribs map[string]float64) map[Category]float64 {
	scores := make(map[Category]float64, len(cs.maxContrib))
	for _, cat := range AllCategories() {
		scores[cat] = cs.ScoreCategory(cat, contribs)
	}
	return scores
}

func (cs *CategoryScorer) CompositeScore(catScores map[Category]float64, catWeights map[Category]float64) float64 {
	var composite float64
	for cat, score := range catScores {
		w := catWeights[cat]
		composite += score * w
	}
	return composite
}

func (cs *CategoryScorer) capContrib(featureID string, contrib float64) float64 {
	cap, ok := cs.caps[featureID]
	if !ok || cap <= 0 {
		return contrib
	}
	w := cs.weights[featureID]
	if w <= 0 {
		return contrib
	}
	maxAllowed := w * cap
	if contrib > maxAllowed {
		return maxAllowed
	}
	return contrib
}
