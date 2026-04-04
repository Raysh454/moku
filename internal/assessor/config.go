package assessor

import "github.com/raysh454/moku/internal/assessor/attacksurface"

// Config holds runtime settings for the assessor.
type Config struct {
	// ScoringVersion allows safe evolution of scoring logic.
	ScoringVersion string `json:"scoring_version"`

	// DefaultConfidence used for no-evidence results.
	DefaultConfidence float64 `json:"default_confidence"`

	ScoreOpts ScoreOptions

	// Saturation controls element count capping for exposure scoring.
	Saturation attacksurface.SaturationConfig `json:"saturation"`
}
