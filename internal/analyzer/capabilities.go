package analyzer

// Capabilities is the static, declarative metadata an Analyzer backend
// publishes about what it supports. Callers (HTTP handlers, UI, orchestrator)
// read Capabilities before building a ScanRequest so they can render correct
// forms and reject unsupported options up front, avoiding round-trips to the
// backend for unsupported features.
//
// All values are static for a given backend instance — calling Capabilities
// MUST NOT perform any network I/O.
type Capabilities struct {
	// Async is true when SubmitScan returns before the scan has finished and
	// GetScan must be polled. Every current backend sets this to true; the
	// field exists for future adapters that may expose a synchronous API.
	Async bool `json:"async"`

	// SupportsAuth indicates whether the backend honors ScanRequest.Auth.
	// When false, callers SHOULD NOT set Auth (it will be ignored).
	SupportsAuth bool `json:"supports_auth"`

	// SupportsScope indicates whether the backend honors ScanRequest.Scope.
	// When false, the scanner will scan whatever crawl strategy it default
	// to without scope filtering.
	SupportsScope bool `json:"supports_scope"`

	// SupportsScanProfile indicates whether the backend maps ScanRequest.Profile
	// to a native scan configuration. When false, Profile is ignored and the
	// backend's default profile is used.
	SupportsScanProfile bool `json:"supports_scan_profile"`

	// MaxConcurrentScans is the backend's self-reported concurrency ceiling.
	// Zero means "unknown" or "unbounded"; callers should treat zero as "do
	// not rate-limit based on this value".
	MaxConcurrentScans int `json:"max_concurrent_scans,omitempty"`

	// Version is the backend's reported version string (e.g. "moku-0.1.0",
	// "Burp Enterprise 2024.5", "ZAP 2.14.0"). Informational.
	Version string `json:"version,omitempty"`
}
