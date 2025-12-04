package analyzer

import (
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/webclient"
)

// ScanRequest represents a request to submit a URL for analysis.
type ScanRequest struct {
	// URL is the target URL to analyze.
	URL string `json:"url"`

	// Options contains optional scan parameters (e.g., depth, timeout).
	Options map[string]string `json:"options,omitempty"`
}

// ScanResult represents the result of an analysis scan.
type ScanResult struct {
	// JobID is the unique identifier for this scan job.
	JobID string `json:"job_id"`

	// Status indicates the current state of the scan (e.g., "pending", "running", "completed", "failed").
	Status string `json:"status"`

	// URL is the target URL that was analyzed.
	URL string `json:"url"`

	// Response contains the HTTP response details if available.
	Response *webclient.Response `json:"response,omitempty"`

	// Score contains the assessment result if scoring was performed.
	Score *assessor.ScoreResult `json:"score,omitempty"`

	// Error contains error details if the scan failed.
	Error string `json:"error,omitempty"`

	// SubmittedAt is when the scan was submitted.
	SubmittedAt time.Time `json:"submitted_at"`

	// CompletedAt is when the scan completed (if applicable).
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Meta contains any additional metadata about the scan.
	Meta map[string]any `json:"meta,omitempty"`
}
