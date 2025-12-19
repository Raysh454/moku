package assessor

import (
	"regexp"
	"time"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

// EvidenceLocation points to a specific part of the document for precise attribution.
// Assessor implementations should populate one or more locations per EvidenceItem when
// RequestLocations is enabled in ScoreOptions.
type EvidenceLocation struct {
	// Type describes what kind of location this is (e.g., "css", "xpath", "header", "cookie").
	// Used by the UI to determine how to interpret and highlight the location.
	Type string `json:"type,omitempty"`

	// Preferred: CSS selector that identifies the element.
	Selector string `json:"selector,omitempty"`

	// XPath expression (optional alternative to CSS selector).
	XPath string `json:"xpath,omitempty"`

	// Regex pattern that matched (if applicable).
	RegexPattern string `json:"regex,omitempty"`

	// Optional file path relative to the site working tree.
	FilePath string `json:"file_path,omitempty"`

	SnapshotID string `json:"snapshot_id,omitempty"`

	// Dom Index is the 0-based index of the element in document.getElementsByTagName("*").
	ParentDOMIndex *int `json:"parent_dom_index,omitempty"`
	DOMIndex       *int `json:"dom_index,omitempty"`

	// Optional byte offsets into the file/body (start inclusive, end exclusive).
	// Useful when the assessor is text-based rather than DOM-aware.
	ByteStart *int `json:"byte_start,omitempty"`
	ByteEnd   *int `json:"byte_end,omitempty"`

	// Optional line numbers (1-based). Useful for working-tree highlighting in the UI.
	LineStart *int `json:"line_start,omitempty"`
	LineEnd   *int `json:"line_end,omitempty"`

	// Line and Column for precise text-based locations.
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`

	// For header-based evidence: the name of the header.
	HeaderName string `json:"header_name,omitempty"`

	// For cookie-based evidence: the name of the cookie.
	CookieName string `json:"cookie_name,omitempty"`

	ParamName string `json:"param_name,omitempty"`

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

	// Contribution is the numeric score contribution of this specific evidence item.
	// Used to explain how each piece of evidence contributed to the overall score.
	Contribution float64 `json:"contribution,omitempty"`
}

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

	// MatchedRules lists rules that matched during evaluation.
	MatchedRules []Rule `json:"matched_rules,omitempty"`

	// Meta contains any additional metadata about the scoring process.
	Meta map[string]any `json:"meta,omitempty"`

	// RawFeatures contains extracted numeric features (featureName -> value), e.g: "has_password_field": 1.0, num_forms: 3.0
	RawFeatures map[string]float64 `json:"raw_features,omitempty"`

	// ContribByRule maps rule IDs to their total contribution to the score.
	// This allows computing rule deltas between versions without re-running the assessor.
	ContribByRule map[string]float64 `json:"contrib_by_rule,omitempty"`

	// Timestamp is the time when this ScoreResult was produced.
	Timestamp time.Time `json:"timestamp"`

	AttackSurface *attacksurface.AttackSurface `json:"attack_surface,omitempty"`
}

// ScoreOptions control scoring behavior and the shape of returned evidence.
type ScoreOptions struct {
	// RequestLocations asks the assessor to populate EvidenceItem.Locations
	// for matching evidence items when possible. If false the assessor may
	// skip expensive DOM location extraction.
	RequestLocations bool

	// (Optional) MaxEvidence controls how many evidence items to return.
	MaxEvidence int

	// Timeout for a scoring operation. (12 Seconds by default)
	Timeout time.Duration

	// MaxRegexEvidenceSamples sets the maximum number of regex matches to process per rule.
	// This prevents excessive processing time for large documents.
	// (Default: 10 Matches)
	MaxRegexEvidenceSamples int

	// MaxRegexMatchLength sets the maximum length for regex matches.
	// This prevents excessive memory usage for large documents.
	// (Default: 200 Characters)
	MaxRegexMatchValueLen int

	// MaxCSSMatches sets the maximum number of CSS selector matches to process per rule.
	// This prevents excessive processing time for large documents.
	// (Default: 10 Matches)
	MaxCSSEvidenceSamples int
}

// Rule defines a single heuristic check the assessor will run.
// Either Selector (CSS) or Regex (PCRE) can be used (both may be set).
type Rule struct {
	ID       string  // unique rule id (eg. "forms:autocomplete-off")
	Key      string  // short key presented in UI (eg. "autocomplete-off")
	Severity string  // "low"|"medium"|"high"|"critical"
	Weight   float64 // numeric weight used to build score
	Selector string  // optional CSS selector to match nodes
	Regex    string  // optional regex pattern (compiled at constructor)
	compiled *regexp.Regexp
}

// ScoreDiff explains how the score changed between two snapshots.
type ScoreDiff struct {
	ScoreBase  float64 `json:"score_base"`
	ScoreHead  float64 `json:"score_head"`
	ScoreDelta float64 `json:"score_delta"`

	// FeatureDeltas: feature -> (head - base)
	FeatureDeltas map[string]float64 `json:"feature_deltas"`

	// RuleDeltas: rule/feature id -> (headContrib - baseContrib)
	// In your current design, rule id == feature name for AttackSurface features.
	RuleDeltas map[string]float64 `json:"rule_deltas"`
}

// Tells us exactly what changed and why between two security snapshots.
// Includes score deltas and attack surface changes.
type SecurityDiff struct {
	FilePath       string `json:"url"`
	BaseSnapshotID string `json:"base_snapshot_id"`
	HeadSnapshotID string `json:"head_snapshot_id"`

	// Score deltas
	ScoreBase  float64 `json:"score_base"`
	ScoreHead  float64 `json:"score_head"`
	ScoreDelta float64 `json:"score_delta"`
	// Optional extra detail
	FeatureDeltas map[string]float64 `json:"feature_deltas,omitempty"`
	RuleDeltas    map[string]float64 `json:"rule_deltas,omitempty"`

	// Attack surface deltas
	AttackSurfaceChanged bool                                `json:"attack_surface_changed"`
	AttackSurfaceChanges []attacksurface.AttackSurfaceChange `json:"attack_surface_changes,omitempty"`
}
