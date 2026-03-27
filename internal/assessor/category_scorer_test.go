package assessor_test

import (
	"math"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
)

func newTestScorer() *assessor.CategoryScorer {
	cfg := assessor.DefaultScoringConfig()
	return assessor.NewCategoryScorer(cfg.FeatureCategories, cfg.FeatureWeights, cfg.FeatureCaps)
}

func TestNewCategoryScorer_ComputesMaxContrib(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()

	for _, cat := range assessor.AllCategories() {
		max := scorer.MaxContrib(cat)
		if max <= 0 {
			t.Errorf("MaxContrib(%q) = %v, expected positive value", cat, max)
		}
	}
}

func TestScoreCategory_EmptyContribs_ReturnsZero(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	contribs := map[string]float64{}

	score := scorer.ScoreCategory(assessor.CategoryHeaders, contribs)
	if score != 0.0 {
		t.Errorf("expected 0.0 for empty contribs, got %v", score)
	}
}

func TestScoreCategory_Headers_AllMissing_ReturnsOne(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()
	scorer := assessor.NewCategoryScorer(cfg.FeatureCategories, cfg.FeatureWeights, cfg.FeatureCaps)

	contribs := map[string]float64{
		"csp_missing":             cfg.FeatureWeights["csp_missing"] * 1.0,
		"csp_unsafe_inline":       cfg.FeatureWeights["csp_unsafe_inline"] * 1.0,
		"csp_unsafe_eval":         cfg.FeatureWeights["csp_unsafe_eval"] * 1.0,
		"xfo_missing":             cfg.FeatureWeights["xfo_missing"] * 1.0,
		"xcto_missing":            cfg.FeatureWeights["xcto_missing"] * 1.0,
		"hsts_missing":            cfg.FeatureWeights["hsts_missing"] * 1.0,
		"referrer_policy_missing": cfg.FeatureWeights["referrer_policy_missing"] * 1.0,
	}

	score := scorer.ScoreCategory(assessor.CategoryHeaders, contribs)
	if score < 0.99 || score > 1.01 {
		t.Errorf("expected ~1.0 when all header features fire, got %v", score)
	}
}

func TestScoreCategory_CapsAreApplied(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()
	scorer := assessor.NewCategoryScorer(cfg.FeatureCategories, cfg.FeatureWeights, cfg.FeatureCaps)

	cap := cfg.FeatureCaps["num_cookies_missing_httponly"]
	weight := cfg.FeatureWeights["num_cookies_missing_httponly"]

	contribAtCap := map[string]float64{
		"num_cookies_missing_httponly": weight * cap,
	}
	contribOverCap := map[string]float64{
		"num_cookies_missing_httponly": weight * (cap + 100),
	}

	scoreAtCap := scorer.ScoreCategory(assessor.CategoryCookies, contribAtCap)
	scoreOverCap := scorer.ScoreCategory(assessor.CategoryCookies, contribOverCap)

	if scoreOverCap > scoreAtCap {
		t.Errorf("score should not increase beyond cap: atCap=%v overCap=%v", scoreAtCap, scoreOverCap)
	}
}

func TestScoreAll_ReturnsAllCategories(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	contribs := map[string]float64{
		"csp_missing": 0.5,
	}

	scores := scorer.ScoreAll(contribs)

	for _, cat := range assessor.AllCategories() {
		if _, ok := scores[cat]; !ok {
			t.Errorf("ScoreAll missing category %q", cat)
		}
	}
}

func TestCompositeScore_AllZeros_ReturnsZero(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	catScores := make(map[assessor.Category]float64)
	for _, cat := range assessor.AllCategories() {
		catScores[cat] = 0.0
	}
	cfg := assessor.DefaultScoringConfig()
	composite := scorer.CompositeScore(catScores, cfg.CategoryWeights)
	if composite != 0.0 {
		t.Errorf("expected 0.0, got %v", composite)
	}
}

func TestCompositeScore_AllOnes_ReturnsOne(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	catScores := make(map[assessor.Category]float64)
	for _, cat := range assessor.AllCategories() {
		catScores[cat] = 1.0
	}
	cfg := assessor.DefaultScoringConfig()
	composite := scorer.CompositeScore(catScores, cfg.CategoryWeights)
	if math.Abs(composite-1.0) > 0.01 {
		t.Errorf("expected ~1.0, got %v", composite)
	}
}

func TestCompositeScore_WeightedAverage(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	cfg := assessor.DefaultScoringConfig()

	catScores := make(map[assessor.Category]float64)
	for _, cat := range assessor.AllCategories() {
		catScores[cat] = 0.0
	}
	catScores[assessor.CategoryHeaders] = 1.0

	composite := scorer.CompositeScore(catScores, cfg.CategoryWeights)
	expected := cfg.CategoryWeights[assessor.CategoryHeaders]

	if math.Abs(composite-expected) > 0.001 {
		t.Errorf("expected %v, got %v", expected, composite)
	}
}

func TestScoreCategory_ReturnsValueInZeroToOne(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	contribs := map[string]float64{
		"csp_missing": 0.5,
		"hsts_missing": 0.2,
	}

	score := scorer.ScoreCategory(assessor.CategoryHeaders, contribs)
	if score < 0.0 || score > 1.0 {
		t.Errorf("score out of bounds: %v", score)
	}
}

func TestRawContrib_ReturnsAccumulatedContribs(t *testing.T) {
	t.Parallel()

	scorer := newTestScorer()
	contribs := map[string]float64{
		"csp_missing":  0.5,
		"hsts_missing": 0.2,
	}

	raw := scorer.RawContrib(assessor.CategoryHeaders, contribs)
	if raw != 0.7 {
		t.Errorf("expected raw contrib 0.7, got %v", raw)
	}
}
