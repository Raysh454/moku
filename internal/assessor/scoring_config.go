package assessor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"gopkg.in/yaml.v2"
)

type ScoringConfig struct {
	FeatureWeights    map[string]float64   `json:"feature_weights" yaml:"feature_weights"`
	CategoryWeights   map[Category]float64 `json:"category_weights" yaml:"category_weights"`
	FeatureCategories map[string]Category  `json:"feature_categories" yaml:"feature_categories"`
	FeatureCaps       map[string]float64   `json:"feature_caps" yaml:"feature_caps"`
	InteractionRules  []InteractionRule    `json:"interaction_rules" yaml:"interaction_rules"`
	ScoreExponent     float64              `json:"score_exponent" yaml:"score_exponent"`
}

func DefaultScoringConfig() *ScoringConfig {
	weights := make(map[string]float64, len(attacksurface.FeatureWeights)+9)
	for k, v := range attacksurface.FeatureWeights {
		weights[k] = v
	}
	for _, r := range DefaultRules() {
		weights[r.ID] = r.Weight
	}

	categories := make(map[string]Category, len(featureCategoryMap))
	for k, v := range featureCategoryMap {
		categories[k] = v
	}

	return &ScoringConfig{
		FeatureWeights: weights,
		CategoryWeights: map[Category]float64{
			CategoryHeaders:    0.25,
			CategoryCookies:    0.10,
			CategoryForms:      0.20,
			CategoryParams:     0.10,
			CategoryScripts:    0.05,
			CategoryDOMHygiene: 0.20,
			CategoryInfoLeak:   0.10,
		},
		FeatureCategories: categories,
		FeatureCaps: map[string]float64{
			"num_cookies_missing_httponly": 5,
			"num_cookies_missing_secure":  5,
			"num_suspicious_params":       5,
			"num_inline_scripts":          10,
			"num_error_indicators":        3,
			"num_password_inputs":         3,
			"num_file_inputs":             3,
			"num_framework_hints":         5,
		},
		InteractionRules: nil,
		ScoreExponent:    1.0,
	}
}

func LoadScoringConfig(path string) (*ScoringConfig, error) {
	if path == "" {
		return nil, errors.New("scoring config: empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &ScoringConfig{}
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, cfg)
	default:
		err = yaml.Unmarshal(data, cfg)
	}

	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func MergeScoringConfig(defaults, overrides *ScoringConfig) *ScoringConfig {
	if overrides == nil {
		return cloneScoringConfig(defaults)
	}

	merged := cloneScoringConfig(defaults)

	for k, v := range overrides.FeatureWeights {
		merged.FeatureWeights[k] = v
	}
	for k, v := range overrides.CategoryWeights {
		merged.CategoryWeights[k] = v
	}
	for k, v := range overrides.FeatureCategories {
		merged.FeatureCategories[k] = v
	}
	for k, v := range overrides.FeatureCaps {
		merged.FeatureCaps[k] = v
	}
	if overrides.InteractionRules != nil {
		merged.InteractionRules = overrides.InteractionRules
	}
	if overrides.ScoreExponent != 0 {
		merged.ScoreExponent = overrides.ScoreExponent
	}

	return merged
}

func cloneScoringConfig(src *ScoringConfig) *ScoringConfig {
	dst := &ScoringConfig{
		ScoreExponent:    src.ScoreExponent,
		FeatureWeights:   make(map[string]float64, len(src.FeatureWeights)),
		CategoryWeights:  make(map[Category]float64, len(src.CategoryWeights)),
		FeatureCategories: make(map[string]Category, len(src.FeatureCategories)),
		FeatureCaps:      make(map[string]float64, len(src.FeatureCaps)),
	}
	for k, v := range src.FeatureWeights {
		dst.FeatureWeights[k] = v
	}
	for k, v := range src.CategoryWeights {
		dst.CategoryWeights[k] = v
	}
	for k, v := range src.FeatureCategories {
		dst.FeatureCategories[k] = v
	}
	for k, v := range src.FeatureCaps {
		dst.FeatureCaps[k] = v
	}
	if src.InteractionRules != nil {
		dst.InteractionRules = make([]InteractionRule, len(src.InteractionRules))
		copy(dst.InteractionRules, src.InteractionRules)
	}
	return dst
}
