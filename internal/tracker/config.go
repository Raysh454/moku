package tracker

// Config controls runtime settings for the tracker scaffold.
type Config struct {
	// StoragePath optional path for on-disk storage (not used by in-memory scaffold).
	StoragePath string `json:"storage_path,omitempty"`

	// MaxHistory limits number of versions to keep in some implementations.
	MaxHistory int `json:"max_history,omitempty"`

	// IDPrefix optional prefix for generated version IDs (helps identify env).
	IDPrefix string `json:"id_prefix,omitempty"`

	// RedactSensitiveHeaders controls whether sensitive headers (Authorization, Cookie, etc.)
	// should be redacted with "[REDACTED]" in snapshots and diffs. Defaults to true if not set.
	RedactSensitiveHeaders *bool `json:"redact_sensitive_headers,omitempty"`
}
