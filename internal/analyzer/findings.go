package analyzer

// Finding is the unified, backend-agnostic representation of a single
// vulnerability or observation produced by a scanner. Every Analyzer backend
// (Moku, Burp Suite, OWASP ZAP, and any future adapter) MUST emit findings in
// this shape.
//
// The fields are the common denominator of what industry scanners natively
// expose: Burp Suite issues (name, severity, confidence, CWE, evidence,
// issue_background, remediation_background) and OWASP ZAP alerts (alert, risk,
// confidence, cweid, wascid, evidence, description, solution, reference).
// Moku's own backend conforms to this shape by mapping the internal assessor
// evidence into Finding entries; any Moku-proprietary extras live in RawData
// under "moku." keys and are NOT part of the public contract.
type Finding struct {
	// ID is a backend-local stable identifier for this finding. Used by
	// consumers to deduplicate results across poll iterations.
	ID string `json:"id"`

	// Title is a human-readable vulnerability name (e.g. "Reflected XSS",
	// "Missing Content-Security-Policy header").
	Title string `json:"title"`

	Severity   Severity   `json:"severity"`
	Confidence Confidence `json:"confidence"`

	// URL is the full affected URL (scheme + host + path + query).
	URL string `json:"url,omitempty"`

	// Path is the path portion of the URL, surfaced separately for grouping
	// findings by route in reporting UIs.
	Path string `json:"path,omitempty"`

	// Method is the HTTP method associated with the finding when applicable
	// (e.g. "POST" for a form-based injection point).
	Method string `json:"method,omitempty"`

	// Parameter is the injection point name when the finding is parameter-
	// scoped: a query parameter, form field, cookie, or header name.
	Parameter string `json:"parameter,omitempty"`

	// CWE lists associated Common Weakness Enumeration IDs. Every major
	// scanner reports these. Moku entries may be empty until the assessor
	// rules are annotated with CWE mappings.
	CWE []int `json:"cwe,omitempty"`

	// WASC lists associated WASC Threat Classification IDs. ZAP reports
	// these natively; Burp and Moku usually leave this empty.
	WASC []int `json:"wasc,omitempty"`

	// Description explains what the finding means in prose.
	Description string `json:"description,omitempty"`

	// Evidence is a redacted request/response snippet or observation that
	// demonstrates why the finding was raised.
	Evidence string `json:"evidence,omitempty"`

	// Remediation is advice on how to fix the underlying weakness.
	Remediation string `json:"remediation,omitempty"`

	// References links to external documentation (OWASP, CVE, vendor
	// advisories) describing the class of vulnerability.
	References []string `json:"references,omitempty"`

	// RawData carries backend-specific extras that do not fit the unified
	// schema. Keys SHOULD be namespaced by backend (e.g. "burp.issue_type_id",
	// "zap.messageId", "moku.contribution"). Consumers that read RawData are
	// explicitly coupling themselves to a specific backend.
	RawData map[string]any `json:"raw_data,omitempty"`
}

// Severity is the canonical severity bucket reported by a finding. The five
// values are the union of Burp's four (Info/Low/Medium/High) with the
// industry-common "Critical" tier.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Confidence reports how confident the scanner is that a finding is real.
// Maps directly to Burp's Tentative/Firm/Certain. ZAP's Low/Medium/High
// normalize to these three values; ZAP's "Confirmed" collapses to Certain.
type Confidence string

const (
	ConfidenceTentative Confidence = "tentative"
	ConfidenceFirm      Confidence = "firm"
	ConfidenceCertain   Confidence = "certain"
)

// ScanStatus is the lifecycle state of an async scan. Every industry scanner
// moves through approximately these states: pending (queued), running (in
// progress), and a terminal state (completed/failed/canceled).
type ScanStatus string

const (
	StatusPending   ScanStatus = "pending"
	StatusRunning   ScanStatus = "running"
	StatusCompleted ScanStatus = "completed"
	StatusFailed    ScanStatus = "failed"
	StatusCanceled  ScanStatus = "canceled"
)

// IsTerminal reports whether s is a terminal scan status — i.e. the scan has
// stopped progressing and callers should stop polling. Useful for the shared
// pollUntilDone helper.
func (s ScanStatus) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}
