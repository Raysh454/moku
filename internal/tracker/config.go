package tracker

// Config controls runtime settings for the tracker scaffold.
type Config struct {
	// StoragePath optional path for on-disk storage (not used by in-memory scaffold).
	StoragePath string `json:"storage_path,omitempty"`

	// RedactSensitiveHeaders controls whether sensitive headers (Authorization, Cookie, etc.)
	// should be redacted with "[REDACTED]" in snapshots and diffs. Defaults to true if not set.
	RedactSensitiveHeaders bool `json:"redact_sensitive_headers,omitempty"`

	// ProjectID, tries to set it if provided.
	// If a projectID already exists, it will not be overwritten, unless force is true.
	ProjectID string `json:"project_id,omitempty"`

	// ForceProjectID forces setting the ProjectID even if one already exists.
	ForceProjectID bool `json:"force_project_id,omitempty"`
}
