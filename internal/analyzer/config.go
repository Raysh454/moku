package analyzer

import "time"

// Backend is the stable identifier of an Analyzer implementation. Values are
// returned by Analyzer.Name() and consumed by the factory switch in
// factory.go. Add a new constant here when introducing a new adapter.
type Backend string

const (
	BackendMoku Backend = "moku"

	// BackendDAST routes scan requests to the Python sidecar's "builtin"
	// adapter, which performs active dynamic-analysis scanning (XSS / SQLi /
	// CSRF probes). The Go side talks HTTP to the sidecar; the actual scan
	// engine lives in services/analyzer/app/adapters/builtin/.
	BackendDAST       Backend = "dast"
	BackendNuclei     Backend = "nuclei"
	BackendNikto      Backend = "nikto"
	BackendShodan     Backend = "shodan"
	BackendVirusTotal Backend = "virustotal"

	// BackendZAP routes through the sidecar's "zap" adapter (which shells
	// out to a local ZAP install). A native Go ZAP client scaffold existed
	// here once but was removed in favor of the working sidecar adapter.
	BackendZAP Backend = "zap"
)

// Config selects the analyzer backend and carries common + per-backend
// settings. Embedded in app.Config as AnalyzerCfg. The shape deliberately
// mirrors webclient.Config so the two plugin points have a consistent
// mental model for contributors.
type Config struct {
	// Backend selects the implementation. Empty defaults to BackendMoku.
	Backend Backend `json:"backend"`

	// DefaultPoll is used by ScanAndWait when callers pass a zero
	// PollOptions. Per-backend overrides can live in the sub-configs below
	// if ever needed.
	DefaultPoll PollOptions `json:"default_poll"`

	// Moku holds settings specific to the Moku backend. Ignored when
	// Backend != BackendMoku.
	Moku MokuConfig `json:"moku"`

	// Sidecar holds settings shared by every backend that routes through the
	// Python analyzer sidecar (DAST/Nuclei/Nikto/Shodan/VirusTotal/ZAP). The
	// adapter-name dispatch happens inside the sidecar — the Go side selects
	// it via the Backend field on each ScanRequest payload.
	Sidecar SidecarConfig `json:"sidecar"`
}

// SidecarConfig carries the connection details for the Python analyzer
// sidecar process (services/analyzer/). One sidecar instance can serve every
// adapter-backed Backend (BackendDAST / BackendNuclei / ...); the per-request
// adapter selection lives in the JSON body sent to /scan.
type SidecarConfig struct {
	// BaseURL is the sidecar root (e.g. "http://127.0.0.1:8181"). Required.
	BaseURL string `json:"base_url"`

	// RequestTimeout bounds each individual HTTP call to the sidecar.
	RequestTimeout time.Duration `json:"request_timeout"`

	// InsecureSkipTLS disables TLS verification when the sidecar is exposed
	// over HTTPS with a self-signed certificate.
	InsecureSkipTLS bool `json:"insecure_skip_tls"`

	// SharedSecret, when non-empty, is sent as the "X-Moku-Token" header
	// on every request. Must match the sidecar's MOKU_ANALYZER_TOKEN env
	// var. Excluded from JSON marshalling so it never round-trips to logs.
	SharedSecret string `json:"-"`
}

// PollOptions controls how ScanAndWait polls the backend for completion.
// Backends share a single pollUntilDone helper that consumes this struct.
type PollOptions struct {
	// Timeout bounds total wait time. Zero falls back to Config.DefaultPoll
	// at the point of use.
	Timeout time.Duration `json:"timeout"`

	// Interval is the initial delay between polls.
	Interval time.Duration `json:"interval"`

	// BackoffFactor multiplies Interval after every poll iteration. 1.0 (or
	// zero, treated as 1.0) means fixed-rate polling; values >1 produce
	// exponential backoff, which Burp's REST API recommends.
	BackoffFactor float64 `json:"backoff_factor"`

	// MaxInterval caps the backoff. Zero means no cap.
	MaxInterval time.Duration `json:"max_interval"`
}

// MokuConfig holds settings for the Moku native backend.
type MokuConfig struct {
	// DefaultProfile is used when ScanRequest.Profile is empty.
	DefaultProfile ScanProfile `json:"default_profile"`

	// JobRetention controls how long a completed or failed scan remains in
	// the in-memory job registry before a background cleanup task removes
	// it. Zero disables cleanup.
	JobRetention time.Duration `json:"job_retention"`
}
