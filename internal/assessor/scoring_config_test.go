package assessor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

func TestDefaultScoringConfig_HasAllFeatureWeights(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()

	for name := range attacksurface.FeatureWeights {
		if _, ok := cfg.FeatureWeights[name]; !ok {
			t.Errorf("DefaultScoringConfig missing feature weight for %q", name)
		}
	}
}

func TestDefaultScoringConfig_HasAllCategoryWeights(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()

	for _, cat := range assessor.AllCategories() {
		w, ok := cfg.CategoryWeights[cat]
		if !ok {
			t.Errorf("DefaultScoringConfig missing category weight for %q", cat)
		}
		if w <= 0 {
			t.Errorf("category weight for %q should be positive, got %v", cat, w)
		}
	}
}

func TestDefaultScoringConfig_CategoryWeightsSumToOne(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()
	var sum float64
	for _, w := range cfg.CategoryWeights {
		sum += w
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("category weights sum to %v, expected ~1.0", sum)
	}
}

func TestDefaultScoringConfig_ScoreExponentIsOne(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()
	if cfg.ScoreExponent != 1.0 {
		t.Errorf("expected default ScoreExponent 1.0, got %v", cfg.ScoreExponent)
	}
}

func TestDefaultScoringConfig_HasFeatureCaps(t *testing.T) {
	t.Parallel()

	cfg := assessor.DefaultScoringConfig()
	expectedCapped := []string{
		"num_cookies_missing_httponly", "num_cookies_missing_secure",
		"num_suspicious_params", "num_inline_scripts",
		"num_error_indicators", "num_password_inputs",
		"num_file_inputs", "num_framework_hints",
	}
	for _, name := range expectedCapped {
		cap, ok := cfg.FeatureCaps[name]
		if !ok {
			t.Errorf("expected FeatureCaps to contain %q", name)
		}
		if cap <= 0 {
			t.Errorf("expected positive cap for %q, got %v", name, cap)
		}
	}
}

func TestMergeScoringConfig_NilOverrides_ReturnsDefaults(t *testing.T) {
	t.Parallel()

	defaults := assessor.DefaultScoringConfig()
	merged := assessor.MergeScoringConfig(defaults, nil)

	if merged.ScoreExponent != defaults.ScoreExponent {
		t.Errorf("expected ScoreExponent %v, got %v", defaults.ScoreExponent, merged.ScoreExponent)
	}
	if len(merged.FeatureWeights) != len(defaults.FeatureWeights) {
		t.Errorf("expected %d feature weights, got %d", len(defaults.FeatureWeights), len(merged.FeatureWeights))
	}
}

func TestMergeScoringConfig_OverridesSpecificWeight(t *testing.T) {
	t.Parallel()

	defaults := assessor.DefaultScoringConfig()
	overrides := &assessor.ScoringConfig{
		FeatureWeights: map[string]float64{
			"csp_missing": 0.99,
		},
	}

	merged := assessor.MergeScoringConfig(defaults, overrides)

	if merged.FeatureWeights["csp_missing"] != 0.99 {
		t.Errorf("expected csp_missing weight 0.99, got %v", merged.FeatureWeights["csp_missing"])
	}
	if merged.FeatureWeights["hsts_missing"] != defaults.FeatureWeights["hsts_missing"] {
		t.Errorf("non-overridden weight should remain default")
	}
}

func TestMergeScoringConfig_OverridesScoreExponent(t *testing.T) {
	t.Parallel()

	defaults := assessor.DefaultScoringConfig()
	overrides := &assessor.ScoringConfig{
		ScoreExponent: 0.7,
	}

	merged := assessor.MergeScoringConfig(defaults, overrides)
	if merged.ScoreExponent != 0.7 {
		t.Errorf("expected ScoreExponent 0.7, got %v", merged.ScoreExponent)
	}
}

func TestLoadScoringConfig_NonexistentPath_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := assessor.LoadScoringConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestLoadScoringConfig_ValidYAML_Loads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "scoring.yaml")

	content := []byte(`feature_weights:
  csp_missing: 0.8
  hsts_missing: 0.3
score_exponent: 0.7
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := assessor.LoadScoringConfig(path)
	if err != nil {
		t.Fatalf("LoadScoringConfig returned error: %v", err)
	}

	if cfg.FeatureWeights["csp_missing"] != 0.8 {
		t.Errorf("expected csp_missing 0.8, got %v", cfg.FeatureWeights["csp_missing"])
	}
	if cfg.FeatureWeights["hsts_missing"] != 0.3 {
		t.Errorf("expected hsts_missing 0.3, got %v", cfg.FeatureWeights["hsts_missing"])
	}
	if cfg.ScoreExponent != 0.7 {
		t.Errorf("expected ScoreExponent 0.7, got %v", cfg.ScoreExponent)
	}
}

func TestLoadScoringConfig_ValidJSON_Loads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "scoring.json")

	content := []byte(`{"feature_weights":{"csp_missing":0.9},"score_exponent":0.5}`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := assessor.LoadScoringConfig(path)
	if err != nil {
		t.Fatalf("LoadScoringConfig returned error: %v", err)
	}

	if cfg.FeatureWeights["csp_missing"] != 0.9 {
		t.Errorf("expected csp_missing 0.9, got %v", cfg.FeatureWeights["csp_missing"])
	}
	if cfg.ScoreExponent != 0.5 {
		t.Errorf("expected ScoreExponent 0.5, got %v", cfg.ScoreExponent)
	}
}

func TestLoadScoringConfig_EmptyPath_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := assessor.LoadScoringConfig("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}
