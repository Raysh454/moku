package fetcher

import (
	"time"

	"github.com/raysh454/moku/internal/filter"
)

// Config holds configuration parameters for the Fetcher.
type Config struct {
	// MaxConcurrency is the number of worker goroutines to spawn for concurrent fetching.
	// This directly controls the worker pool size - exactly MaxConcurrency workers will
	// be created, regardless of the number of URLs to fetch.
	//
	// Example values:
	//   - 5-10:  Good for rate-limited APIs or slow networks
	//   - 20-50: Balanced performance for most use cases
	//   - 100+:  High throughput for fast, unrestricted endpoints
	MaxConcurrency int

	// CommitSize is the number of snapshots to batch before committing to the tracker.
	// Larger batches reduce database overhead but increase memory usage.
	CommitSize int

	// ScoreTimeout is the maximum time to wait for scoring a version.
	ScoreTimeout time.Duration
}

// FetchOptions contains options for a fetch operation, including filter configuration.
type FetchOptions struct {
	// FilterConfig is the filter configuration to apply during fetching.
	// If nil, no filtering is applied.
	FilterConfig *filter.FilterConfig

	// FilterStatusCodes controls whether to filter responses by status code.
	// When true, responses matching SkipStatusCodes in FilterConfig are filtered out.
	FilterStatusCodes bool
}
