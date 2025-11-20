package model

import "time"

// EvidenceItem is one piece of evidence contributing to a score.
type EvidenceItem struct {
	// Key is a short identifier for the evidence (e.g. "has-iframe", "suspicious-link").
	Key string `json:"key"`

	// Severity is a human-level severity bucket such as "low", "medium", "high".
	Severity string `json:"severity"`

	// Description is a short human-readable explanation of the evidence.
	Description string `json:"description"`

	// Value contains the raw value that triggered the evidence (string/number/map...)
	Value any `json:"value,omitempty"`

	// RuleID identifies the rule that produced this evidence.
	RuleID string `json:"rule_id,omitempty"`
}

// ScoreResult is the canonical assessor output for a single document/URL.
type ScoreResult struct {
	// Score is the normalized internal score range [0.0 .. 1.0].
	Score float64 `json:"score"`

	// Normalized is an integer normalized form [0 .. 100] for ease of reporting.
	Normalized int `json:"normalized"`

	// Confidence is the assessor's confidence [0.0 .. 1.0] in this result.
	Confidence float64 `json:"confidence"`

	// Version identifies the scoring algorithm / heuristics version used.
	Version string `json:"version"`

	// Evidence is the list of contributing evidence items.
	Evidence []EvidenceItem `json:"evidence,omitempty"`

	// MatchedRules lists IDs of rules that matched during evaluation.
	MatchedRules []string `json:"matched_rules,omitempty"`

	// RawFeatures contains extracted numeric features (featureName -> value).
	RawFeatures map[string]float64 `json:"raw_features,omitempty"`

	// Meta contains any auxiliary metadata (source, timing hints, etc).
	Meta map[string]any `json:"meta,omitempty"`

	// Timestamp is the time when this ScoreResult was produced.
	Timestamp time.Time `json:"timestamp"`
}
