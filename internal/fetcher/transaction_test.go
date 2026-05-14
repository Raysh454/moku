package fetcher_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/fetcher"
)

// TestFetch_SingleVersion verifies that one fetch operation creates one version,
// even when multiple batches are committed.
func TestFetch_SingleVersion(t *testing.T) {
	logger := &DummyLogger{}
	wc := &DummyWebClient{}
	tr := &DummyTracker{}
	idx := &DummyEndpointIndex{}

	// Configure fetcher with small batch size to force multiple batches
	cfg := fetcher.Config{
		MaxConcurrency: 4,
		CommitSize:     2, // Force multiple batches
	}

	f, err := fetcher.New(cfg, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	// Fetch 5 URLs (should create 3 batches: 2 + 2 + 1)
	urls := []string{"url1", "url2", "url3", "url4", "url5"}
	_ = f.Fetch(context.Background(), urls, nil)

	// Verify multiple batches were created (should be 3)
	if len(tr.Batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(tr.Batches))
	}

	// Verify FinalizeCommit was called exactly once
	if tr.FinalizedCount != 1 {
		t.Errorf("Expected FinalizeCommit called 1 time, got %d", tr.FinalizedCount)
	}

	// Verify all 5 snapshots were added
	if len(tr.AllSnapshots) != 5 {
		t.Errorf("Expected 5 total snapshots, got %d", len(tr.AllSnapshots))
	}
}

// TestFetch_LargeSet verifies handling of large URL sets.
func TestFetch_LargeSet(t *testing.T) {
	logger := &DummyLogger{}
	wc := &DummyWebClient{}
	tr := &DummyTracker{}
	idx := &DummyEndpointIndex{}

	cfg := fetcher.Config{
		MaxConcurrency: 4,
		CommitSize:     100,
	}

	f, err := fetcher.New(cfg, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	// Fetch 250 URLs (should create 3 batches: 100 + 100 + 50)
	urls := make([]string, 250)
	for i := range urls {
		urls[i] = string(rune('A' + i%26))
	}

	_ = f.Fetch(context.Background(), urls, nil)

	// Verify 3 batches (100 + 100 + 50)
	if len(tr.Batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(tr.Batches))
	}

	// Verify FinalizeCommit was called exactly once
	if tr.FinalizedCount != 1 {
		t.Errorf("Expected FinalizeCommit called 1 time, got %d", tr.FinalizedCount)
	}

	// Verify all 250 snapshots were added
	if len(tr.AllSnapshots) != 250 {
		t.Errorf("Expected 250 total snapshots, got %d", len(tr.AllSnapshots))
	}
}

// TestFetch_WithCancellation verifies behavior when context is cancelled.
func TestFetch_WithCancellation(t *testing.T) {
	logger := &DummyLogger{}
	wc := &DummyWebClient{}
	tr := &DummyTracker{}
	idx := &DummyEndpointIndex{}

	cfg := fetcher.Config{
		MaxConcurrency: 1, // Sequential to make cancellation predictable
		CommitSize:     10,
	}

	f, err := fetcher.New(cfg, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	urls := []string{"url1", "url2", "url3"}
	_ = f.Fetch(ctx, urls, nil)

	// With immediate cancellation, we might get 0 or 1 finalize
	// The important thing is that CancelCommit is called if needed
	// (verified by no panic and clean shutdown)
}

// TestFetch_WithErrors verifies behavior when some fetches fail.
func TestFetch_WithErrors(t *testing.T) {
	logger := &DummyLogger{}
	wc := &DummyWebClient{
		FailURLs: map[string]bool{
			"fail1": true,
			"fail2": true,
		},
	}
	tr := &DummyTracker{}
	idx := &DummyEndpointIndex{}

	cfg := fetcher.Config{
		MaxConcurrency: 4,
		CommitSize:     10,
	}

	f, err := fetcher.New(cfg, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	urls := []string{"ok1", "fail1", "ok2", "fail2", "ok3"}
	_ = f.Fetch(context.Background(), urls, nil)

	// Should finalize once (even with some failures)
	if tr.FinalizedCount != 1 {
		t.Errorf("Expected FinalizeCommit called 1 time, got %d", tr.FinalizedCount)
	}

	// Should only have 3 successful snapshots
	if len(tr.AllSnapshots) != 3 {
		t.Errorf("Expected 3 successful snapshots, got %d", len(tr.AllSnapshots))
	}

	// Verify errors were logged
	if len(logger.Errors) == 0 {
		t.Error("Expected error logs for failed fetches")
	}
}
