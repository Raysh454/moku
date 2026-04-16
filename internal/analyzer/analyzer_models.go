package analyzer

import "time"

// ScanRequest describes a URL to scan plus universal options common to
// industry scanners. Backend-specific tweaks live in RawOptions under
// namespaced keys (e.g. "burp.scan_configuration", "zap.context_id").
//
// Fields marked "ignored when Capabilities.X is false" are silently dropped
// by backends that don't advertise the capability; callers should consult
// Capabilities before populating these fields.
type ScanRequest struct {
	// URL is the primary scan target. Required.
	URL string `json:"url"`

	// Scope narrows which hosts and paths the scanner may crawl. Ignored
	// when Capabilities.SupportsScope is false.
	Scope *ScanScope `json:"scope,omitempty"`

	// Auth carries credentials the scanner should use to authenticate
	// against the target. Ignored when Capabilities.SupportsAuth is false.
	// Credential fields (Password, Token) are excluded from JSON marshalling
	// so this struct never accidentally round-trips secrets through logs
	// or HTTP responses.
	Auth *ScanAuth `json:"auth,omitempty"`

	// Profile is a backend-agnostic selector mapped by each adapter to its
	// native scan configuration (Burp scan_configurations preset, ZAP
	// ascan policy, Moku evidence depth). Ignored when
	// Capabilities.SupportsScanProfile is false.
	Profile ScanProfile `json:"profile,omitempty"`

	// MaxDuration is a soft time budget for the scan. Zero means "use the
	// backend default". Backends may honor this approximately.
	MaxDuration time.Duration `json:"max_duration,omitempty"`

	// RawOptions is a per-backend escape hatch for options that do not fit
	// the universal schema. Keys MUST be namespaced by backend name, e.g.
	// "burp.scan_configuration_name" or "zap.context_id".
	RawOptions map[string]string `json:"raw_options,omitempty"`
}

// ScanScope expresses include/exclude rules for what the scanner may traverse.
// Patterns are scanner-native regex; Moku does not validate them because
// different backends use different regex dialects.
type ScanScope struct {
	IncludeHosts []string `json:"include_hosts,omitempty"`
	ExcludeHosts []string `json:"exclude_hosts,omitempty"`
	IncludePaths []string `json:"include_paths,omitempty"`
	ExcludePaths []string `json:"exclude_paths,omitempty"`
}

// ScanAuth carries credentials for authenticated scanning. Secret fields are
// tagged json:"-" so they never leak through marshalling.
type ScanAuth struct {
	// Type selects the authentication scheme the scanner should use.
	// Supported values: "none", "basic", "bearer", "form".
	Type string `json:"type"`

	Username string `json:"username,omitempty"`
	Password string `json:"-"`
	Token    string `json:"-"`

	// Extra holds scheme-specific parameters (e.g. form field selectors for
	// Type=="form"). Keys and semantics are scanner-defined.
	Extra map[string]string `json:"extra,omitempty"`
}

// ScanProfile is a backend-agnostic intensity selector. Each adapter maps it
// to a native concept: Burp maps to a scan_configurations preset, ZAP maps to
// an ascan policy, Moku maps to evidence-extraction depth.
type ScanProfile string

const (
	ProfileQuick    ScanProfile = "quick"
	ProfileBalanced ScanProfile = "balanced"
	ProfileThorough ScanProfile = "thorough"
)

// ScanResult is the shape every Analyzer backend returns from GetScan and
// ScanAndWait. Modeled on what industry scanners natively expose
// (Burp Enterprise's scan task status + Burp issues; ZAP's scan status +
// alerts). There are no backend-specific fields at the top level — Moku-only
// extras (exposure score, hardening score, attack surface) live under
// RawData["moku.*"] keys when the Moku backend chooses to emit them.
//
// Contract: once Status is StatusCompleted, Findings MUST be non-nil (may be
// empty) and Summary MUST be non-nil.
type ScanResult struct {
	// JobID is the backend-local scan identifier (Burp's task_id, ZAP's
	// scan ID, Moku's UUID).
	JobID string `json:"job_id"`

	// Backend identifies which adapter produced this result, mirroring the
	// value returned by Analyzer.Name().
	Backend Backend `json:"backend"`

	Status ScanStatus `json:"status"`

	// URL is the target that was scanned.
	URL string `json:"url"`

	// Error carries a human-readable failure message. Populated only when
	// Status == StatusFailed.
	Error string `json:"error,omitempty"`

	SubmittedAt time.Time  `json:"submitted_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress is an optional mid-run progress snapshot. Backends MAY
	// populate this while Status is StatusRunning. Consumers MUST tolerate
	// a nil Progress at any status.
	Progress *ScanProgress `json:"progress,omitempty"`

	// Findings is the unified, backend-agnostic list of results. MUST be
	// non-nil (possibly empty) once Status is StatusCompleted. This is what
	// consumers read.
	Findings []Finding `json:"findings"`

	// Summary aggregates per-severity counts. Mirrors Burp's issue_counts
	// and ZAP's per-risk totals. MUST be non-nil once Status is
	// StatusCompleted and its counts MUST sum to len(Findings).
	Summary *ScanSummary `json:"summary,omitempty"`

	// RawData is a backend-specific escape hatch. Keys MUST be namespaced
	// by backend name (e.g. "burp.task_id", "zap.scan_id",
	// "moku.exposure_score"). Reading RawData explicitly couples the caller
	// to a specific backend and is NOT part of the LSP contract.
	RawData map[string]any `json:"raw_data,omitempty"`
}

// ScanProgress reports a scanner's self-estimated progress. Percent is in
// [0, 100] when known; -1 indicates indeterminate progress (e.g. spider
// phase of ZAP before the URL tree is enumerated).
type ScanProgress struct {
	Percent int    `json:"percent"`
	Phase   string `json:"phase,omitempty"`
	Note    string `json:"note,omitempty"`
}

// ScanSummary holds per-severity counts of findings in a completed scan.
// Redundant with Findings but cheap to compute and present in nearly every
// scanner API (Burp returns issue_counts; ZAP's alert-summary endpoint
// returns totals per risk). Per-severity counts MUST sum to Total.
type ScanSummary struct {
	Total    int `json:"total"`
	Info     int `json:"info"`
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
}
