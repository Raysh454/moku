package model

import "time"

// EvidenceLocation points to a specific part of the document for precise attribution.
// Assessor implementations should populate one or more locations per EvidenceItem when
// RequestLocations is enabled in ScoreOptions.
type EvidenceLocation struct {
	// Preferred: CSS selector that identifies the element.
	Selector string `json:"selector,omitempty"`

	// Alternative: XPath for precise DOM targeting.
	XPath string `json:"xpath,omitempty"`

	// Optional node id or DOM-specific identifier.
	NodeID string `json:"node_id,omitempty"`

	// Optional file path relative to the site working tree (if multi-file snapshot).
	FilePath string `json:"file_path,omitempty"`

	// Optional byte offsets into the file/body (start inclusive, end exclusive).
	// Useful when the assessor is text-based rather than DOM-aware.
	ByteStart *int `json:"byte_start,omitempty"`
	ByteEnd   *int `json:"byte_end,omitempty"`

	// Optional line numbers (1-based). Useful for working-tree highlighting in the UI.
	LineStart *int `json:"line_start,omitempty"`
	LineEnd   *int `json:"line_end,omitempty"`

	// Optional per-location confidence/weight (0..1). When present, attribution
	// code should use these to split evidence weight across locations.
	Confidence *float64 `json:"confidence,omitempty"`

	// Optional human note about this specific location (e.g., "in modal dialog").
	Note string `json:"note,omitempty"`
}

// EvidenceItem is one piece of evidence produced by an assessor rule.
// An EvidenceItem can reference multiple concrete locations on the page.
type EvidenceItem struct {
	// ID is an optional stable identifier for this evidence item in the ScoreResult.
	// Useful for attribution rows to reference an evidence item unambiguously.
	ID string `json:"id,omitempty"`

	// Key is a short identifier for the evidence (e.g. "has-iframe", "suspicious-link").
	Key string `json:"key"`

	// RuleID identifies the rule that produced this evidence.
	RuleID string `json:"rule_id,omitempty"`

	// Severity is a human-level severity bucket such as "low", "medium", "high", "critical".
	Severity string `json:"severity"`

	// Description is a short human-readable explanation of the evidence.
	Description string `json:"description"`

	// Value contains the raw value that triggered the evidence (string/number/map...).
	Value any `json:"value,omitempty"`

	// Locations holds zero or more structured locators where this evidence was observed.
	// When non-empty it enables exact attribution and deterministic UI highlighting.
	Locations []EvidenceLocation `json:"locations,omitempty"`
}

// ScoreResult is the canonical assessor output for a single document/URL.
// Example:
//
//	{
//	  "score": 0.7,
//	  "version": "heuristics-v1",
//	  "confidence": 0.9,
//	  "evidence": [
//	    {
//	      "id": "ev-1",
//	      "key": "insecure-form",
//	      "rule_id": "forms:autocomplete-off",
//	      "severity": "high",
//	      "description": "Login form has autocomplete disabled",
//	      "value": "<form id=\"login\">",
//	      "locations": [
//	        {"selector":"form#login", "line_start":10, "line_end":12, "confidence":1.0}
//	      ]
//	    },
//	    {
//	      "id": "ev-2",
//	      "key": "style-change",
//	      "rule_id": "ui:style-inline",
//	      "severity": "low",
//	      "description": "Inline style added",
//	      "locations": [
//	        {"selector":"style", "line_start":12, "line_end":13, "confidence":0.5}
//	      ]
//	    }
//	  ]
//	}
type ScoreResult struct {
	// Score is the normalized internal score range [0.0 .. 1.0].
	Score float64 `json:"score"`

	// Normalized is an integer normalized form [0 .. 100] for ease of reporting.
	Normalized int `json:"normalized"`

	// Confidence is the assessor's confidence [0.0 .. 1.0] in this result.
	Confidence float64 `json:"confidence"`

	// Version identifies the scoring algorithm / heuristics version used.
	// This should map to the assessor's ruleset version (for auditability).
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

// ScoreOptions control scoring behavior and the shape of returned evidence.
type ScoreOptions struct {
	// RequestLocations asks the assessor to populate EvidenceItem.Locations
	// for matching evidence items when possible. If false the assessor may
	// skip expensive DOM location extraction.
	RequestLocations bool

	// Lightweight mode asks assessor to run a cheap, fast pass (may produce
	// less evidence). Use for low-latency paths.
	Lightweight bool

	// (Optional) MaxEvidence controls how many evidence items to return.
	MaxEvidence int

	// Timeout for a scoring operation. (12 Seconds by default)
	Timeout time.Duration
}
