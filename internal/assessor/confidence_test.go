package assessor_test

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor"
)

func TestComputeConfidence_FullFeatures_HighConfidence(t *testing.T) {
	t.Parallel()

	features := make(map[string]float64)
	for i := 0; i < 44; i++ {
		features[string(rune('a'+i))] = 1.0
	}
	ruleContribs := map[string]float64{"dom:inline-event-handler": 0.3}

	conf := assessor.ComputeConfidence(features, ruleContribs, 5000, 200)
	if conf < 0.8 {
		t.Errorf("expected high confidence (>=0.8), got %v", conf)
	}
}

func TestComputeConfidence_EmptyFeatures_LowConfidence(t *testing.T) {
	t.Parallel()

	conf := assessor.ComputeConfidence(map[string]float64{}, map[string]float64{}, 0, 0)
	if conf > 0.1 {
		t.Errorf("expected low confidence (<=0.1), got %v", conf)
	}
}

func TestComputeConfidence_SmallBody_ReducedConfidence(t *testing.T) {
	t.Parallel()

	features := make(map[string]float64)
	for i := 0; i < 10; i++ {
		features[string(rune('a'+i))] = 1.0
	}

	confSmall := assessor.ComputeConfidence(features, map[string]float64{}, 30, 200)
	confLarge := assessor.ComputeConfidence(features, map[string]float64{}, 5000, 200)

	if confSmall >= confLarge {
		t.Errorf("small body should have lower confidence: small=%v large=%v", confSmall, confLarge)
	}
}

func TestComputeConfidence_ErrorStatus_ReducedConfidence(t *testing.T) {
	t.Parallel()

	features := make(map[string]float64)
	for i := 0; i < 10; i++ {
		features[string(rune('a'+i))] = 1.0
	}

	conf200 := assessor.ComputeConfidence(features, map[string]float64{}, 1000, 200)
	conf500 := assessor.ComputeConfidence(features, map[string]float64{}, 1000, 500)

	if conf500 >= conf200 {
		t.Errorf("error status should have lower confidence: 200=%v 500=%v", conf200, conf500)
	}
}

func TestComputeConfidence_NeverExceedsOne(t *testing.T) {
	t.Parallel()

	features := make(map[string]float64)
	for i := 0; i < 100; i++ {
		features[string(rune(i))] = 1.0
	}
	ruleContribs := map[string]float64{"a": 1, "b": 1, "c": 1}

	conf := assessor.ComputeConfidence(features, ruleContribs, 100000, 200)
	if conf > 1.0 {
		t.Errorf("confidence should not exceed 1.0, got %v", conf)
	}
}
