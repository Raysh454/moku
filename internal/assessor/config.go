package assessor

// Config holds runtime settings for the assessor. Keep small initially.
type Config struct {
	// ScoringVersion allows safe evolution of scoring logic.
	ScoringVersion string `json:"scoring_version"`

	// DefaultConfidence used for no-evidence results.
	DefaultConfidence float64 `json:"default_confidence"`

	// Rules is the default set of DOM heuristic rules.
	// When NewHeuristicsAssessor receives nil rules, it falls back to this field.
	Rules []Rule `json:"-"`

	ScoreOpts ScoreOptions
}
