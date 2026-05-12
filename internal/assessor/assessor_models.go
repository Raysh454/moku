package assessor

import (
	"time"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

type Snapshot struct {
	ID         string
	VersionID  string
	StatusCode int
	URL        string
	Headers    map[string][]string
	Body       []byte
}

// EvidenceLocation points to a specific part of the document for precise attribution.
// Assessor implementations should populate one or more locations per EvidenceItem when
// RequestLocations is enabled in ScoreOptions.
type EvidenceLocation struct {
	// Type describes what kind of location this is (e.g., "header", "cookie", "input", "form").
	// Used by the UI to determine how to interpret and highlight the location.
	Type string `json:"type,omitempty"`

	// XPath expression (optional alternative to CSS selector).
	XPath string `json:"xpath,omitempty"`

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
	// Score combines exposure and hardening: exposure * (1 - hardening) [0..inf).
	Score float64 `json:"score"`

	// SnapshotID is the snapshot this score applies to.
	SnapshotID string `json:"snapshot_id"`

	// VersionID is the version this score applies to.
	VersionID string `json:"version_id"`

	// Normalized is an integer normalized form [0 .. 100] for ease of reporting.
	Normalized int `json:"normalized"`

	// Confidence is the assessor's confidence [0.0 .. 1.0] in this result.
	Confidence float64 `json:"confidence"`

	// Version identifies the scoring algorithm / heuristics version used.
	Version string `json:"version"`

	// Evidence is the list of contributing evidence items.
	Evidence []EvidenceItem `json:"evidence,omitempty"`

	// Meta contains any additional metadata about the scoring process.
	Meta map[string]any `json:"meta,omitempty"`

	// ExposureScore measures how much attack surface the page exposes [0..inf).
	ExposureScore float64 `json:"exposure_score"`

	// HardeningScore measures how well security headers defend the page [0..1].
	HardeningScore float64 `json:"hardening_score"`

	// Timestamp is the time when this ScoreResult was produced.
	Timestamp time.Time `json:"timestamp"`

	AttackSurface *attacksurface.AttackSurface `json:"attack_surface,omitempty"`
}

// ComputeScore computes the score from exposure and hardening.
// Score = Exposure * (1 - Hardening)
func ComputeScore(exposure, hardening float64) float64 {
	return exposure * (1.0 - hardening)
}

// ScoreOptions control scoring behavior and the shape of returned evidence.
type ScoreOptions struct {
	// RequestLocations asks the assessor to populate EvidenceItem.Locations
	// for attack surface features when possible. If false the assessor may
	// skip location extraction for performance.
	RequestLocations bool

	// (Optional) MaxEvidence controls how many evidence items to return.
	MaxEvidence int
}

// ScoreDiff explains how the score changed between two snapshots.
type ScoreDiff struct {
	ScoreBase  float64 `json:"score_base"`
	ScoreHead  float64 `json:"score_head"`
	ScoreDelta float64 `json:"score_delta"`

	ExposureDelta  float64 `json:"exposure_delta"`
	HardeningDelta float64 `json:"hardening_delta"`
}

// Tells us exactly what changed and why between two security snapshots.
// Includes score deltas and attack surface changes.
type SecurityDiff struct {
	FilePath       string `json:"url"`
	BaseVersionID  string `json:"base_version_id"`
	HeadVersionID  string `json:"head_version_id"`
	BaseSnapshotID string `json:"base_snapshot_id"`
	HeadSnapshotID string `json:"head_snapshot_id"`

	// Score deltas
	ScoreBase  float64 `json:"score_base"`
	ScoreHead  float64 `json:"score_head"`
	ScoreDelta float64 `json:"score_delta"`

	// Per-axis score deltas. ExposureDelta is head.ExposureScore - base.ExposureScore
	// (positive = more attack surface exposed). HardeningDelta is
	// head.HardeningScore - base.HardeningScore (positive = better defenses).
	ExposureDelta  float64 `json:"exposure_delta"`
	HardeningDelta float64 `json:"hardening_delta"`

	// Attack surface deltas
	AttackSurfaceChanged bool                                `json:"attack_surface_changed"`
	AttackSurfaceChanges []attacksurface.AttackSurfaceChange `json:"attack_surface_changes,omitempty"`
}

type SecurityDiffOverview struct {
	BaseVersionID string                      `json:"base_version_id"`
	HeadVersionID string                      `json:"head_version_id"`
	Entries       []SecurityDiffOverviewEntry `json:"entries"`
}

type SecurityDiffOverviewEntry struct {
	FilePath                string  `json:"url"`
	BaseSnapshotID          string  `json:"base_snapshot_id,omitempty"`
	HeadSnapshotID          string  `json:"head_snapshot_id,omitempty"`
	ScoreBase               float64 `json:"score_base"`
	ScoreHead               float64 `json:"score_head"`
	ScoreDelta              float64 `json:"score_delta"`
	ExposureDelta           float64 `json:"exposure_delta"`
	HardeningDelta          float64 `json:"hardening_delta"`
	AttackSurfaceChanged    bool    `json:"attack_surface_changed"`
	NumAttackSurfaceChanges int     `json:"num_attack_surface_changes"`
	// (scoreDelta > 0) for quick UI signals
	Regressed bool `json:"regressed"`
}
