package assessor

// Config holds runtime settings for the assessor. Keep small initially.
type Config struct {
	// ScoringVersion allows safe evolution of scoring logic.
	ScoringVersion string `json:"scoring_version"`

	// DefaultConfidence used for no-evidence results.
	DefaultConfidence float64 `json:"default_confidence"`

	// RuleWeights allows configuring weights per rule id (optional).
	RuleWeights map[string]float64 `json:"rule_weights"`
}
