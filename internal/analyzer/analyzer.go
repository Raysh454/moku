// Package analyzer defines a pluggable vulnerability-scanner interface and
// the built-in Moku backend. Additional adapters (Burp Suite, OWASP ZAP, and
// future integrations) implement the same Analyzer interface and are selected
// at runtime via analyzer.Config.Backend.
//
// The interface is shaped by what industry-standard scanners natively expose
// (async submission, progress polling, unified finding model with severity +
// confidence + CWE) rather than by Moku's internal assessor shape. Moku's own
// backend conforms to the interface like any other adapter; there are no
// Moku-specific fields at the top level of ScanRequest or ScanResult.
package analyzer

import "context"

// Analyzer is the pluggable interface every vulnerability scanner backend
// implements. Implementations MUST be safe for concurrent use by multiple
// goroutines; the orchestrator calls these methods from handlers serving
// different HTTP requests concurrently.
//
// Lifecycle contract:
//   - SubmitScan returns quickly with a backend-generated job ID; the scan
//     itself runs asynchronously.
//   - GetScan returns the current state. Once Status is StatusCompleted,
//     the returned ScanResult's Findings and Summary MUST be non-nil.
//   - ScanAndWait submits a scan and polls GetScan until terminal status,
//     timeout, or context cancellation.
//   - Close releases any resources held by the backend and MUST be safe to
//     call more than once.
type Analyzer interface {
	// Name returns the stable Backend identifier of this implementation.
	// The value MUST match a constant declared in config.go.
	Name() Backend

	// Capabilities returns a static, declarative snapshot of what this
	// backend supports. Callers consult it before populating optional
	// ScanRequest fields (Scope, Auth, Profile).
	Capabilities() Capabilities

	// SubmitScan enqueues a scan and returns a backend-local job ID. The
	// call MUST return promptly — it does not block on scan completion.
	// Returns an error when req is nil, req.URL is empty, or the backend
	// cannot accept the submission.
	SubmitScan(ctx context.Context, req *ScanRequest) (string, error)

	// GetScan returns the current state of the scan identified by jobID.
	// Returns an error when jobID is empty or unknown to the backend.
	GetScan(ctx context.Context, jobID string) (*ScanResult, error)

	// ScanAndWait submits a scan and polls GetScan until the scan reaches a
	// terminal status, the poll timeout elapses, or ctx is canceled.
	// When opts fields are zero, the backend falls back to its configured
	// default poll options.
	ScanAndWait(ctx context.Context, req *ScanRequest, opts PollOptions) (*ScanResult, error)

	// Health returns a short status string ("ok", "degraded", "unavailable")
	// and a nil error when the backend is reachable. A non-nil error means
	// the backend is definitively unavailable.
	Health(ctx context.Context) (string, error)

	// Close releases any resources held by the backend (HTTP clients, job
	// registries, background goroutines). MUST be idempotent.
	Close() error
}
