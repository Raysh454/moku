package analyzer

import "time"

// Backend is the stable identifier of an Analyzer implementation. Values are
// returned by Analyzer.Name() and consumed by the factory switch in
// factory.go. Add a new constant here when introducing a new adapter.
type Backend string

const (
	BackendMoku Backend = "moku"
	BackendBurp Backend = "burp"
	BackendZAP  Backend = "zap"
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

	// Burp holds settings specific to the Burp backend. Ignored when
	// Backend != BackendBurp.
	Burp BurpConfig `json:"burp"`

	// ZAP holds settings specific to the ZAP backend. Ignored when
	// Backend != BackendZAP.
	ZAP ZAPConfig `json:"zap"`
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

// BurpConfig holds settings for the Burp Suite Enterprise adapter.
// The adapter is not implemented in this plan; the config exists so
// downstream plans can add it without touching the factory signature.
type BurpConfig struct {
	// BaseURL is the Burp REST API root (e.g. "https://burp.local:1337").
	BaseURL string `json:"base_url"`

	// APIKey is the Burp API key. Excluded from JSON marshalling.
	APIKey string `json:"-"`

	// ScanConfigName is the name of a pre-configured Burp scan
	// configuration. Mapped from ScanRequest.Profile by the adapter.
	ScanConfigName string `json:"scan_config_name,omitempty"`

	// RequestTimeout bounds each individual HTTP call to the Burp API.
	RequestTimeout time.Duration `json:"request_timeout"`

	// InsecureSkipTLS disables TLS verification when talking to self-hosted
	// Burp instances with self-signed certs. Off by default.
	InsecureSkipTLS bool `json:"insecure_skip_tls"`
}

// ZAPConfig holds settings for the OWASP ZAP adapter.
// The adapter is not implemented in this plan.
type ZAPConfig struct {
	// BaseURL is the ZAP REST API root (e.g. "http://127.0.0.1:8090").
	BaseURL string `json:"base_url"`

	// APIKey is the ZAP API key. ZAP accepts the key as a query parameter;
	// the adapter's HTTP helper handles this quirk.
	APIKey string `json:"-"`

	// ContextName selects a pre-configured ZAP context (authenticated
	// session, scope rules). Optional.
	ContextName string `json:"context_name,omitempty"`

	// RequestTimeout bounds each HTTP call to the ZAP API.
	RequestTimeout time.Duration `json:"request_timeout"`

	// InsecureSkipTLS disables TLS verification when talking to ZAP
	// instances with self-signed certs.
	InsecureSkipTLS bool `json:"insecure_skip_tls"`

	// SpiderMaxDepth caps the spider phase depth.
	SpiderMaxDepth int `json:"spider_max_depth,omitempty"`

	// AscanPolicy is the name of a pre-configured ZAP active-scan policy.
	// Mapped from ScanRequest.Profile by the adapter.
	AscanPolicy string `json:"ascan_policy,omitempty"`
}
