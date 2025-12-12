package fetcher_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/webclient"
)

//
// ───────────────────────────────────────────────
//   Dummy Implementations
// ───────────────────────────────────────────────
//

// Dummy WebClient — returns body "ok:<url>" unless FailURLs[url] = true
type DummyWebClient struct {
	ResponseDelay time.Duration
	FailURLs      map[string]bool
}

func (d *DummyWebClient) Do(ctx context.Context, req *webclient.Request) (*webclient.Response, error) {
	if d.ResponseDelay > 0 {
		time.Sleep(d.ResponseDelay)
	}
	if d.FailURLs != nil && d.FailURLs[req.URL] {
		return nil, errors.New("dummy fetch fail")
	}

	return &webclient.Response{
		Request:    req,
		Body:       []byte("ok:" + req.URL),
		StatusCode: 200,
		FetchedAt:  time.Now(),
	}, nil
}

func (d *DummyWebClient) Get(ctx context.Context, url string) (*webclient.Response, error) {
	return d.Do(ctx, &webclient.Request{Method: "GET", URL: url})
}

func (d *DummyWebClient) Close() error { return nil }

// Dummy Tracker
type DummyTracker struct {
	mu      sync.Mutex
	Batches [][]*tracker.Snapshot
}

func (t *DummyTracker) Commit(ctx context.Context, snap *tracker.Snapshot, message, author string) (*tracker.CommitResult, error) {
	return &tracker.CommitResult{}, nil
}

func (t *DummyTracker) CommitBatch(ctx context.Context, snaps []*tracker.Snapshot, message, author string) ([]*tracker.CommitResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	copySnaps := append([]*tracker.Snapshot(nil), snaps...)
	t.Batches = append(t.Batches, copySnaps)

	results := make([]*tracker.CommitResult, len(snaps))
	for i := range snaps {
		results[i] = &tracker.CommitResult{}
	}
	return results, nil
}

func (t *DummyTracker) ScoreAndAttributeVersion(ctx context.Context, cr *tracker.CommitResult) error {
	return nil
}

func (t *DummyTracker) SetAssessor(a assessor.Assessor) {
	// no-op for dummy
}

func (t *DummyTracker) Diff(ctx context.Context, baseID, headID string) (*tracker.DiffResult, error) {
	return nil, nil
}

func (t *DummyTracker) Get(ctx context.Context, versionID string) ([]*tracker.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) List(ctx context.Context, limit int) ([]*tracker.Version, error) {
	return nil, nil
}

func (t *DummyTracker) Checkout(ctx context.Context, versionID string) error {
	return nil
}

func (t *DummyTracker) Close() error { return nil }

// Dummy Logger implementing the full Logger interface
type DummyLogger struct {
	mu     sync.Mutex
	Errors []string
	Infos  []string
	Debugs []string
	Warns  []string
}

func (l *DummyLogger) Debug(msg string, fields ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Debugs = append(l.Debugs, msg)
}

func (l *DummyLogger) Info(msg string, fields ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Infos = append(l.Infos, msg)
}

func (l *DummyLogger) Warn(msg string, fields ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Warns = append(l.Warns, msg)
}

func (l *DummyLogger) Error(msg string, fields ...logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Errors = append(l.Errors, msg)
}

func (l *DummyLogger) With(fields ...logging.Field) logging.Logger {
	// For simplicity, just return itself.
	return l
}

//
// ───────────────────────────────────────────────
//   TESTS
// ───────────────────────────────────────────────
//

func TestFetcher_Batching(t *testing.T) {
	ctx := context.Background()

	tracker := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}

	f, err := fetcher.New(5, 2, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"1", "2", "3", "4", "5"}
	f.Fetch(ctx, urls)

	expected := []int{2, 2, 1}

	if len(tracker.Batches) != len(expected) {
		t.Fatalf("expected %d batches, got %d", len(expected), len(tracker.Batches))
	}

	for i, size := range expected {
		if got := len(tracker.Batches[i]); got != size {
			t.Fatalf("batch %d expected %d snapshots, got %d", i, size, got)
		}
	}
}

func TestFetcher_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tracker := &DummyTracker{}
	wc := &DummyWebClient{ResponseDelay: 50 * time.Millisecond}
	logger := &DummyLogger{}

	f, err := fetcher.New(10, 3, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"one", "two", "three", "four"}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel() // cancel while fetching
	}()

	f.Fetch(ctx, urls)

	if len(tracker.Batches) > 1 {
		t.Fatalf("expected at most 1 batch due to cancellation, got %d", len(tracker.Batches))
	}
}

func TestFetcher_LogsFetchErrors(t *testing.T) {
	ctx := context.Background()

	tracker := &DummyTracker{}
	wc := &DummyWebClient{FailURLs: map[string]bool{"bad": true}}
	logger := &DummyLogger{}

	f, err := fetcher.New(5, 2, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	f.Fetch(ctx, []string{"a", "bad", "b"})

	if len(logger.Errors) == 0 {
		t.Fatalf("expected logged errors but got none")
	}
}

func TestFetcher_FetchResponseBodies(t *testing.T) {
	ctx := context.Background()

	tracker := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}

	f, err := fetcher.New(5, 2, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"x", "y", "z"}
	f.Fetch(ctx, urls)

	// Check that snapshot bodies match "ok:<url>"
	found := map[string]bool{}
	for _, batch := range tracker.Batches {
		for _, snap := range batch {
			if string(snap.Body) != "ok:"+snap.URL {
				t.Errorf("unexpected snapshot body: %s", string(snap.Body))
			}
			found[snap.URL] = true
		}
	}

	for _, u := range urls {
		if !found[u] {
			t.Errorf("missing snapshot for url %s", u)
		}
	}
}

func TestFetcher_FinalBatchFlush(t *testing.T) {
	ctx := context.Background()

	tracker := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}

	f, err := fetcher.New(5, 3, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"a", "b", "c", "d"} // 4 snapshots, commit size = 3
	f.Fetch(ctx, urls)

	expectedBatches := 2 // one batch of 3, one batch of 1
	if len(tracker.Batches) != expectedBatches {
		t.Fatalf("expected %d batches, got %d", expectedBatches, len(tracker.Batches))
	}

	// Ensure last batch has the remaining snapshot
	lastBatch := tracker.Batches[len(tracker.Batches)-1]
	if len(lastBatch) != 1 {
		t.Errorf("expected last batch size 1, got %d", len(lastBatch))
	}
}

func TestFetcher_ConcurrentFetchSafety(t *testing.T) {
	ctx := context.Background()

	tracker := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}

	f, err := fetcher.New(20, 5, tracker, wc, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Fire multiple Fetch calls concurrently
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			urls := []string{fmt.Sprintf("u%d-1", i), fmt.Sprintf("u%d-2", i)}
			f.Fetch(ctx, urls)
		}(i)
	}
	wg.Wait()

	// All snapshots should be committed
	total := 0
	for _, b := range tracker.Batches {
		total += len(b)
	}
	if total != 6 {
		t.Errorf("expected 6 snapshots total, got %d", total)
	}
}
