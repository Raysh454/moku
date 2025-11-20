package interfaces

import (
	"context"

	"github.com/raysh454/moku/internal/model"
)

// Analyzer is the interface for submitting and managing analysis scans.
// Implementations may perform synchronous or asynchronous analysis of URLs,
// returning scan results that include HTTP response details and scoring data.
//
// This interface defines the contract for analyzer services, allowing the
// rest of the codebase to depend on an abstraction rather than concrete types.
type Analyzer interface {
	// SubmitScan submits a URL for analysis and returns a job ID.
	// The scan may be processed asynchronously; use GetScan to retrieve results.
	SubmitScan(ctx context.Context, req *model.ScanRequest) (string, error)

	// GetScan retrieves the status and results of a previously submitted scan.
	GetScan(ctx context.Context, jobID string) (*model.ScanResult, error)

	// ScanAndWait submits a scan and polls for completion within the specified timeout.
	// timeoutSec is the maximum time to wait for completion (in seconds).
	// pollIntervalSec is how frequently to check for completion (in seconds).
	ScanAndWait(ctx context.Context, req *model.ScanRequest, timeoutSec int, pollIntervalSec int) (*model.ScanResult, error)

	// Health checks if the analyzer service is healthy and ready to accept requests.
	// Returns a status message or error if the service is unavailable.
	Health(ctx context.Context) (string, error)

	// Close releases any resources held by the analyzer.
	Close() error
}
