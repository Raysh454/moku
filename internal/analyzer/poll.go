package analyzer

import (
	"context"
	"fmt"
	"time"
)

// pollUntilDone is the shared polling helper every backend uses to implement
// ScanAndWait. It repeatedly calls a.GetScan(jobID) until the scan reaches a
// terminal status, the timeout elapses, or ctx is canceled. Applying the
// Template Method pattern at function level (rather than subclass) is the
// idiomatic Go shape.
//
// Step A status: skeleton. Step B fleshes out the backoff logic and wires the
// Moku backend to call this helper from ScanAndWait.
func pollUntilDone(ctx context.Context, a Analyzer, jobID string, opts PollOptions) (*ScanResult, error) {
	if a == nil {
		return nil, fmt.Errorf("pollUntilDone: nil analyzer")
	}
	if jobID == "" {
		return nil, fmt.Errorf("pollUntilDone: empty job ID")
	}

	interval := opts.Interval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	backoff := opts.BackoffFactor
	if backoff < 1 {
		backoff = 1
	}
	maxInterval := opts.MaxInterval

	deadline := time.Time{}
	if opts.Timeout > 0 {
		deadline = time.Now().Add(opts.Timeout)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := a.GetScan(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if result != nil && result.Status.IsTerminal() {
			return result, nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, fmt.Errorf("pollUntilDone: timed out waiting for job %q", jobID)
		}

		sleep := interval
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining < sleep {
				sleep = remaining
			}
		}
		if sleep > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}
		}

		// Apply backoff for the next iteration.
		if backoff > 1 {
			next := time.Duration(float64(interval) * backoff)
			if maxInterval > 0 && next > maxInterval {
				next = maxInterval
			}
			interval = next
		}
	}
}
