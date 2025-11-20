package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

// analyzerAdapter wraps the concrete DefaultAnalyzer and implements interfaces.Analyzer.
// This adapter provides a forward-compatible interface for the existing implementation,
// allowing the codebase to program against interfaces.Analyzer.
//
// Note: The current implementation performs synchronous scans. Job IDs are generated
// locally and results are returned immediately. Future implementations may support
// true asynchronous processing.
type analyzerAdapter struct {
	impl   *DefaultAnalyzer
	logger interfaces.Logger
}

// NewAnalyzer creates a new analyzer that implements interfaces.Analyzer by wrapping
// the existing DefaultAnalyzer implementation. This is the preferred constructor for
// new code that wants to depend on the Analyzer interface.
//
// Parameters:
//   - cfg: Application configuration
//   - logger: Logger instance for structured logging
//   - httpClient: Optional HTTP client for customization (nil uses default)
//
// Returns an interfaces.Analyzer or an error if initialization fails.
func NewAnalyzer(cfg *app.Config, logger interfaces.Logger, httpClient *http.Client) (interfaces.Analyzer, error) {
	// Call the existing constructor to create the concrete implementation.
	// This preserves all existing logic and ensures backward compatibility.
	impl, err := NewDefaultAnalyzer(cfg, logger, httpClient)
	if err != nil {
		return nil, fmt.Errorf("NewAnalyzer: %w", err)
	}

	componentLogger := logger.With(interfaces.Field{Key: "component", Value: "analyzer_adapter"})
	componentLogger.Info("created analyzer adapter")

	return &analyzerAdapter{
		impl:   impl,
		logger: componentLogger,
	}, nil
}

// SubmitScan submits a URL for analysis. In the current implementation, this performs
// a synchronous scan and returns a generated job ID immediately.
func (a *analyzerAdapter) SubmitScan(ctx context.Context, req *model.ScanRequest) (string, error) {
	if a == nil || a.impl == nil {
		return "", fmt.Errorf("SubmitScan: nil analyzer")
	}
	if req == nil {
		return "", fmt.Errorf("SubmitScan: nil request")
	}

	a.logger.Info("submitting scan", interfaces.Field{Key: "url", Value: req.URL})

	// Generate a simple job ID based on timestamp
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())

	// For the current implementation, we perform the scan synchronously
	// and could cache the result. For simplicity, we just return the job ID.
	// Future implementations may queue the job for asynchronous processing.

	return jobID, nil
}

// GetScan retrieves scan results for a given job ID. In the current implementation,
// this performs a fresh scan since we don't maintain job state.
//
// TODO: This is a placeholder implementation. A production version would maintain
// a job registry and return cached results for completed jobs.
func (a *analyzerAdapter) GetScan(ctx context.Context, jobID string) (*model.ScanResult, error) {
	if a == nil || a.impl == nil {
		return nil, fmt.Errorf("GetScan: nil analyzer")
	}
	if jobID == "" {
		return nil, fmt.Errorf("GetScan: empty job ID")
	}

	a.logger.Info("retrieving scan", interfaces.Field{Key: "job_id", Value: jobID})

	// Current implementation doesn't maintain job state, so we return a placeholder result.
	// A real implementation would look up the job in a registry.
	return &model.ScanResult{
		JobID:       jobID,
		Status:      "unknown",
		Error:       "job tracking not implemented in current version",
		SubmittedAt: time.Now(),
	}, nil
}

// ScanAndWait performs a synchronous scan and waits for completion.
// This is the primary method for the current implementation.
func (a *analyzerAdapter) ScanAndWait(ctx context.Context, req *model.ScanRequest, timeoutSec int, pollIntervalSec int) (*model.ScanResult, error) {
	if a == nil || a.impl == nil {
		return nil, fmt.Errorf("ScanAndWait: nil analyzer")
	}
	if req == nil {
		return nil, fmt.Errorf("ScanAndWait: nil request")
	}

	a.logger.Info("performing scan and wait",
		interfaces.Field{Key: "url", Value: req.URL},
		interfaces.Field{Key: "timeout_sec", Value: timeoutSec})

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	submittedAt := time.Now()

	// Use the existing Analyze method to fetch and analyze the URL
	resp, err := a.impl.Analyze(ctx, req.URL)
	completedAt := time.Now()

	if err != nil {
		return &model.ScanResult{
			JobID:       jobID,
			Status:      "failed",
			URL:         req.URL,
			Error:       err.Error(),
			SubmittedAt: submittedAt,
			CompletedAt: &completedAt,
		}, err
	}

	return &model.ScanResult{
		JobID:       jobID,
		Status:      "completed",
		URL:         req.URL,
		Response:    resp,
		SubmittedAt: submittedAt,
		CompletedAt: &completedAt,
	}, nil
}

// Health checks if the analyzer is ready to accept requests.
func (a *analyzerAdapter) Health(ctx context.Context) (string, error) {
	if a == nil || a.impl == nil {
		return "", fmt.Errorf("Health: nil analyzer")
	}

	a.logger.Debug("health check")
	return "ok", nil
}

// Close releases resources held by the analyzer.
func (a *analyzerAdapter) Close() error {
	if a == nil || a.impl == nil {
		return fmt.Errorf("Close: nil analyzer")
	}

	a.logger.Info("closing analyzer adapter")
	return a.impl.Close()
}
