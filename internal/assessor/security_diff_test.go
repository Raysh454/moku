package assessor

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

func TestNewSecurityDiff_PropagatesScoresAndChanges(t *testing.T) {
	baseScore := &ScoreResult{
		Score:         0.2,
		RawFeatures:   map[string]float64{"a": 1},
		ContribByRule: map[string]float64{"a": 0.1},
	}
	headScore := &ScoreResult{
		Score:         0.5,
		RawFeatures:   map[string]float64{"a": 2},
		ContribByRule: map[string]float64{"a": 0.3},
	}

	baseAS := &attacksurface.AttackSurface{
		Forms: []attacksurface.Form{{Action: "/login", Method: "POST"}},
	}
	headAS := &attacksurface.AttackSurface{
		Forms: []attacksurface.Form{{Action: "/login", Method: "POST"}, {Action: "/admin", Method: "POST"}},
	}

	d, err := NewSecurityDiff("https://example.com/path", "base-1", "head-1", baseScore, headScore, baseAS, headAS)
	if err != nil {
		t.Fatalf("NewSecurityDiff returned error: %v", err)
	}

	if d.FilePath != "/path" {
		t.Errorf("expected FilePath == /path, got %q", d.FilePath)
	}
	if d.ScoreBase != 0.2 || d.ScoreHead != 0.5 || d.ScoreDelta != 0.3 {
		t.Errorf("unexpected scores: base=%v head=%v delta=%v", d.ScoreBase, d.ScoreHead, d.ScoreDelta)
	}
	if len(d.FeatureDeltas) == 0 {
		t.Errorf("expected non-empty FeatureDeltas, got %v", d.FeatureDeltas)
	}
	if len(d.RuleDeltas) == 0 {
		t.Errorf("expected non-empty RuleDeltas, got %v", d.RuleDeltas)
	}
	if !d.AttackSurfaceChanged || len(d.AttackSurfaceChanges) == 0 {
		t.Errorf("expected attack surface changes to be reported, got changed=%v changes=%v", d.AttackSurfaceChanged, d.AttackSurfaceChanges)
	}
}

func TestNewSecurityDiff_InvalidURL(t *testing.T) {
	_, err := NewSecurityDiff("://bad url", "base", "head", nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error for invalid URL, got nil")
	}
}
