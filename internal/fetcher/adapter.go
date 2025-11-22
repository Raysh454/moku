package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/raysh454/moku/internal/model"
)

// TrackerCommitter is an interface for committing snapshots to a tracker.
// This allows us to use either Commit or CommitBatch without importing concrete tracker types.
type TrackerCommitter interface {
	Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.Version, error)
}

// BatchTrackerCommitter extends TrackerCommitter with batch commit capability.
type BatchTrackerCommitter interface {
	TrackerCommitter
	CommitBatch(ctx context.Context, snapshots []*model.Snapshot, message string, author string) ([]*model.Version, error)
}

// CommitResponseToTracker reads an HTTP response and commits it as a snapshot to the tracker.
// This adapter function bridges the fetcher (which works with HTTP responses) and the tracker
// (which works with model.Snapshot instances).
//
// Parameters:
//   - ctx: Context for the operation
//   - tr: Tracker instance (must implement TrackerCommitter)
//   - resp: HTTP response to commit
//   - suggestedPath: Suggested file path for the snapshot (e.g., "index.html")
//
// Returns:
//   - *model.Version: The created version
//   - error: Any error encountered during the commit
//
// The function:
// 1. Reads the response body
// 2. Extracts headers from the response
// 3. Constructs a model.Snapshot with URL, Body, Headers (in Meta), StatusCode, and timestamp
// 4. Calls tracker.Commit to store the snapshot
// 5. Does NOT write any files to disk (tracker handles persistence)
func CommitResponseToTracker(ctx context.Context, tr TrackerCommitter, resp *http.Response, suggestedPath string) (*model.Version, error) {
	if tr == nil {
		return nil, fmt.Errorf("tracker cannot be nil")
	}
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()

	// Extract headers from response
	headers := make(map[string][]string)
	for name, values := range resp.Header {
		headers[name] = values
	}

	// Serialize headers to JSON for storage in Meta
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Get URL from request
	url := ""
	if resp.Request != nil && resp.Request.URL != nil {
		url = resp.Request.URL.String()
	}

	// Build metadata map
	meta := map[string]string{
		"_headers":     string(headersJSON),
		"_status_code": fmt.Sprintf("%d", resp.StatusCode),
		"_file_path":   suggestedPath,
	}

	// Construct snapshot
	snapshot := &model.Snapshot{
		URL:       url,
		Body:      body,
		Meta:      meta,
		CreatedAt: time.Now(),
	}

	// Commit to tracker
	message := fmt.Sprintf("Fetched %s", url)
	version, err := tr.Commit(ctx, snapshot, message, "fetcher")
	if err != nil {
		return nil, fmt.Errorf("failed to commit snapshot: %w", err)
	}

	return version, nil
}

// CommitResponseBatchToTracker commits multiple HTTP responses as a batch to the tracker.
// This is more efficient than individual commits when fetching multiple pages.
//
// Parameters:
//   - ctx: Context for the operation
//   - tr: Tracker instance (must implement BatchTrackerCommitter)
//   - responses: Map of file path to HTTP response
//   - message: Commit message for the batch
//   - author: Author of the commit
//
// Returns:
//   - []*model.Version: The created versions
//   - error: Any error encountered during the commit
func CommitResponseBatchToTracker(ctx context.Context, tr BatchTrackerCommitter, responses map[string]*http.Response, message string, author string) ([]*model.Version, error) {
	if tr == nil {
		return nil, fmt.Errorf("tracker cannot be nil")
	}
	if len(responses) == 0 {
		return nil, fmt.Errorf("no responses to commit")
	}

	snapshots := make([]*model.Snapshot, 0, len(responses))

	for path, resp := range responses {
		if resp == nil {
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for %s: %w", path, err)
		}
		defer resp.Body.Close()

		// Extract headers from response
		headers := make(map[string][]string)
		for name, values := range resp.Header {
			headers[name] = values
		}

		// Serialize headers to JSON for storage in Meta
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal headers for %s: %w", path, err)
		}

		// Get URL from request
		url := ""
		if resp.Request != nil && resp.Request.URL != nil {
			url = resp.Request.URL.String()
		}

		// Build metadata map
		meta := map[string]string{
			"_headers":     string(headersJSON),
			"_status_code": fmt.Sprintf("%d", resp.StatusCode),
			"_file_path":   path,
		}

		// Construct snapshot
		snapshot := &model.Snapshot{
			URL:       url,
			Body:      body,
			Meta:      meta,
			CreatedAt: time.Now(),
		}

		snapshots = append(snapshots, snapshot)
	}

	// Use default message if not provided
	if message == "" {
		message = fmt.Sprintf("Fetched %d pages", len(snapshots))
	}
	if author == "" {
		author = "fetcher"
	}

	// Commit batch to tracker
	versions, err := tr.CommitBatch(ctx, snapshots, message, author)
	if err != nil {
		return nil, fmt.Errorf("failed to commit batch: %w", err)
	}

	return versions, nil
}
