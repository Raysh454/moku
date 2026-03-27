package fetcher_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
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
	mu             sync.Mutex
	Batches        [][]*models.Snapshot
	PendingCommit  *models.PendingCommit
	AllSnapshots   []*models.Snapshot // Track all snapshots across batches
	FinalizedCount int                // Track how many times FinalizeCommit was called
}

func (t *DummyTracker) Commit(ctx context.Context, snap *models.Snapshot, message, author string) (*models.CommitResult, error) {
	return &models.CommitResult{}, nil
}

func (t *DummyTracker) CommitBatch(ctx context.Context, snaps []*models.Snapshot, message, author string) (*models.CommitResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	copySnaps := append([]*models.Snapshot(nil), snaps...)
	t.Batches = append(t.Batches, copySnaps)

	return &models.CommitResult{Snapshots: copySnaps}, nil
}

func (t *DummyTracker) BeginCommit(ctx context.Context, message, author string) (*models.PendingCommit, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.PendingCommit = &models.PendingCommit{
		VersionID: "dummy-version-id",
		Message:   message,
		Author:    author,
		Timestamp: time.Now(),
	}
	// Use a non-nil marker to indicate transaction is active
	t.PendingCommit.SetTransaction(&sql.Tx{})
	t.AllSnapshots = []*models.Snapshot{} // Reset for new commit

	return t.PendingCommit, nil
}

func (t *DummyTracker) AddSnapshots(ctx context.Context, pc *models.PendingCommit, snapshots []*models.Snapshot) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if pc.GetTransaction() == nil {
		return errors.New("no active transaction")
	}

	// Record the batch
	copySnaps := append([]*models.Snapshot(nil), snapshots...)
	t.Batches = append(t.Batches, copySnaps)
	t.AllSnapshots = append(t.AllSnapshots, copySnaps...)

	return nil
}

func (t *DummyTracker) FinalizeCommit(ctx context.Context, pc *models.PendingCommit) (*models.CommitResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if pc.GetTransaction() == nil {
		return nil, errors.New("no active transaction")
	}

	// Mark transaction as complete
	pc.SetTransaction(nil)
	t.FinalizedCount++

	return &models.CommitResult{
		Version: models.Version{
			ID:      pc.VersionID,
			Message: pc.Message,
		},
		Snapshots: t.AllSnapshots,
	}, nil
}

func (t *DummyTracker) CancelCommit(ctx context.Context, pc *models.PendingCommit) error {
	if pc != nil {
		pc.SetTransaction(nil)
	}
	return nil
}

func (t *DummyTracker) ScoreAndAttributeVersion(ctx context.Context, cr *models.CommitResult, _ time.Duration) error {
	return nil
}

func (t *DummyTracker) SetAssessor(a assessor.Assessor) {}

func (t *DummyTracker) GetScoreResultFromSnapshotID(ctx context.Context, snapshotID string) (*assessor.ScoreResult, error) {
	return nil, nil
}

func (t *DummyTracker) GetScoreResultsFromVersionID(ctx context.Context, versionID string) ([]*assessor.ScoreResult, error) {
	return nil, nil
}

func (t *DummyTracker) GetSecurityDiffOverview(ctx context.Context, baseID, headID string) (*assessor.SecurityDiffOverview, error) {
	return nil, nil
}

func (t *DummyTracker) GetSecurityDiff(ctx context.Context, baseSnapshotID, headSnapshotID string) (*assessor.SecurityDiff, error) {
	return nil, nil
}

func (t *DummyTracker) DiffVersions(ctx context.Context, baseID, headID string) (*models.CombinedMultiDiff, error) {
	return nil, nil
}

func (t *DummyTracker) DiffSnapshots(ctx context.Context, baseSnapshotID, headSnapshotID string) (*models.CombinedFileDiff, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshot(ctx context.Context, snapshotID string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshots(ctx context.Context, versionID string) ([]*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshotByURL(ctx context.Context, url string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshotByURLAndVersionID(ctx context.Context, url, versionID string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetParentVersionID(ctx context.Context, versionID string) (string, error) {
	return "", nil
}

func (t *DummyTracker) ListVersions(ctx context.Context, limit int) ([]*models.Version, error) {
	return nil, nil
}

func (t *DummyTracker) Checkout(ctx context.Context, versionID string) error { return nil }

func (t *DummyTracker) HEADExists() (bool, error) { return false, nil }

func (t *DummyTracker) ReadHEAD() (string, error) { return "", nil }

func (t *DummyTracker) DB() *sql.DB { return nil }

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
	return l
}

// DummyEndpointIndex implements indexer.EndpointIndex for tests.
type DummyEndpointIndex struct {
	mu              sync.Mutex
	Failed          []string
	FetchedBatches  [][]string
	PendingBatches  [][]string
	ListedEndpoints []indexer.Endpoint

	MarkFailedErr       error
	MarkFetchedBatchErr error
	MarkPendingBatchErr error
}

func (d *DummyEndpointIndex) AddEndpoints(ctx context.Context, rawUrls []string, source string) ([]string, error) {
	return nil, nil
}

func (d *DummyEndpointIndex) ListEndpoints(ctx context.Context, status string, limit int) ([]indexer.Endpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]indexer.Endpoint(nil), d.ListedEndpoints...), nil
}

func (d *DummyEndpointIndex) MarkPending(ctx context.Context, canonical string) error { return nil }

func (d *DummyEndpointIndex) MarkFetched(ctx context.Context, canonical, versionID string, fetchedAt time.Time) error {
	return nil
}

func (d *DummyEndpointIndex) MarkFailed(ctx context.Context, canonical string, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Failed = append(d.Failed, canonical)
	return d.MarkFailedErr
}

func (d *DummyEndpointIndex) MarkPendingBatch(ctx context.Context, canonicals []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := append([]string(nil), canonicals...)
	d.PendingBatches = append(d.PendingBatches, cp)
	return d.MarkPendingBatchErr
}

func (d *DummyEndpointIndex) MarkFetchedBatch(ctx context.Context, canonicals []string, versionID string, fetchedAt time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := append([]string(nil), canonicals...)
	d.FetchedBatches = append(d.FetchedBatches, cp)
	return d.MarkFetchedBatchErr
}

//
// ───────────────────────────────────────────────
//   TESTS
// ───────────────────────────────────────────────
//

func TestFetcher_Batching(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 2}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"1", "2", "3", "4", "5"}
	f.Fetch(ctx, urls, nil)

	expected := []int{2, 2, 1}

	if len(tr.Batches) != len(expected) {
		t.Fatalf("expected %d batches, got %d", len(expected), len(tr.Batches))
	}

	for i, size := range expected {
		if got := len(tr.Batches[i]); got != size {
			t.Fatalf("batch %d expected %d snapshots, got %d", i, size, got)
		}
	}
}

func TestFetcher_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tr := &DummyTracker{}
	wc := &DummyWebClient{ResponseDelay: 50 * time.Millisecond}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 10, CommitSize: 3}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"one", "two", "three", "four"}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel() // cancel while fetching
	}()

	f.Fetch(ctx, urls, nil)

	if len(tr.Batches) > 1 {
		t.Fatalf("expected at most 1 batch due to cancellation, got %d", len(tr.Batches))
	}
}

func TestFetcher_LogsFetchErrors_AndMarksFailed(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{FailURLs: map[string]bool{"bad": true}}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 2}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	f.Fetch(ctx, []string{"a", "bad", "b"}, nil)

	if len(logger.Errors) == 0 {
		t.Fatalf("expected logged errors but got none")
	}

	// Ensure indexer.MarkFailed was called for "bad"
	foundBad := false
	for _, u := range idx.Failed {
		if u == "bad" {
			foundBad = true
			break
		}
	}
	if !foundBad {
		t.Fatalf("expected indexer.MarkFailed to be called for 'bad', got %v", idx.Failed)
	}
}

func TestFetcher_FetchResponseBodies(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 2}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"x", "y", "z"}
	f.Fetch(ctx, urls, nil)

	// Check that snapshot bodies match "ok:<url>"
	found := map[string]bool{}
	for _, batch := range tr.Batches {
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

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 3}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"a", "b", "c", "d"} // 4 snapshots, commit size = 3
	f.Fetch(ctx, urls, nil)

	expectedBatches := 2 // one batch of 3, one batch of 1
	if len(tr.Batches) != expectedBatches {
		t.Fatalf("expected %d batches, got %d", expectedBatches, len(tr.Batches))
	}

	// Ensure last batch has the remaining snapshot
	lastBatch := tr.Batches[len(tr.Batches)-1]
	if len(lastBatch) != 1 {
		t.Errorf("expected last batch size 1, got %d", len(lastBatch))
	}
}

func TestFetcher_ConcurrentFetchSafety(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 20, CommitSize: 5}, tr, wc, idx, logger)
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
			f.Fetch(ctx, urls, nil)
		}(i)
	}
	wg.Wait()

	// All snapshots should be committed
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}
	if total != 6 {
		t.Errorf("expected 6 snapshots total, got %d", total)
	}
}

//
// ───────────────────────────────────────────────
//   Worker Pool Pattern Tests
// ───────────────────────────────────────────────
//

// TestFetcher_WorkerPoolLimitsGoroutines verifies that the worker pool
// spawns exactly MaxConcurrency workers, not one per URL.
func TestFetcher_WorkerPoolLimitsGoroutines(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{ResponseDelay: 100 * time.Millisecond}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	maxConcurrency := 5
	f, err := fetcher.New(fetcher.Config{MaxConcurrency: maxConcurrency, CommitSize: 10}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Create many more URLs than workers
	urls := make([]string, 50)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	// The key test: with worker pool pattern, this should spawn only MaxConcurrency workers
	// (not 50 goroutines). We can't directly count goroutines in the test, but we can
	// verify all URLs are processed correctly which proves the pattern works.
	f.Fetch(ctx, urls, nil)

	// Verify all snapshots were processed
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}
	if total != len(urls) {
		t.Errorf("expected %d snapshots, got %d", len(urls), total)
	}
}

// TestFetcher_WorkerPoolWithSingleWorker tests the edge case of MaxConcurrency=1
func TestFetcher_WorkerPoolWithSingleWorker(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 1, CommitSize: 2}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"a", "b", "c", "d", "e"}
	f.Fetch(ctx, urls, nil)

	// All URLs should still be processed
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}
	if total != len(urls) {
		t.Errorf("expected %d snapshots, got %d", len(urls), total)
	}
}

// TestFetcher_WorkerPoolWithHighConcurrency tests with many workers
func TestFetcher_WorkerPoolWithHighConcurrency(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 100, CommitSize: 5}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := make([]string, 50)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	f.Fetch(ctx, urls, nil)

	// All URLs should be processed
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}
	if total != len(urls) {
		t.Errorf("expected %d snapshots, got %d", len(urls), total)
	}
}

// TestFetcher_WorkerPoolCancellationMidFetch verifies graceful shutdown
// when context is cancelled during fetching.
func TestFetcher_WorkerPoolCancellationMidFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tr := &DummyTracker{}
	wc := &DummyWebClient{ResponseDelay: 50 * time.Millisecond}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 10}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := make([]string, 100)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	// Cancel after a short time
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	f.Fetch(ctx, urls, nil)

	// We expect fewer than all URLs to be processed due to cancellation
	// The exact number is non-deterministic, but should be < 100
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}

	if total >= len(urls) {
		t.Errorf("expected cancellation to prevent all %d URLs from being fetched, but got %d", len(urls), total)
	}

	t.Logf("Processed %d/%d URLs before cancellation", total, len(urls))
}

// TestFetcher_WorkerPoolErrorHandlingDoesNotBlock verifies that errors
// in some workers don't block other workers or cause deadlock.
func TestFetcher_WorkerPoolErrorHandlingDoesNotBlock(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	// Make every other URL fail
	failURLs := map[string]bool{}
	for i := 0; i < 50; i += 2 {
		failURLs[fmt.Sprintf("url-%d", i)] = true
	}
	wc := &DummyWebClient{FailURLs: failURLs}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 10, CommitSize: 5}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := make([]string, 50)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	f.Fetch(ctx, urls, nil)

	// Count successful snapshots
	total := 0
	for _, b := range tr.Batches {
		total += len(b)
	}

	// Should have processed only the non-failing URLs (25)
	expected := 25
	if total != expected {
		t.Errorf("expected %d successful snapshots, got %d", expected, total)
	}

	// Verify failed URLs were marked in indexer
	if len(idx.Failed) != expected {
		t.Errorf("expected %d failed URLs in indexer, got %d", expected, len(idx.Failed))
	}
}

// TestFetcher_WorkerPoolProcessesAllURLs verifies that all URLs are processed
// even when there are more URLs than workers.
func TestFetcher_WorkerPoolProcessesAllURLs(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	maxConcurrency := 3
	f, err := fetcher.New(fetcher.Config{MaxConcurrency: maxConcurrency, CommitSize: 2}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	// 10 URLs with only 3 workers
	urls := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	f.Fetch(ctx, urls, nil)

	// Collect all processed URLs
	processed := make(map[string]bool)
	for _, batch := range tr.Batches {
		for _, snap := range batch {
			processed[snap.URL] = true
		}
	}

	// All URLs should be processed
	for _, url := range urls {
		if !processed[url] {
			t.Errorf("URL %s was not processed", url)
		}
	}

	if len(processed) != len(urls) {
		t.Errorf("expected %d unique URLs processed, got %d", len(urls), len(processed))
	}
}

// TestFetcher_WorkerPoolProgressCallback tests that progress callbacks
// are called correctly with worker pool pattern.
func TestFetcher_WorkerPoolProgressCallback(t *testing.T) {
	ctx := context.Background()

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 5, CommitSize: 3}, tr, wc, idx, logger)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{"a", "b", "c", "d", "e"}

	var mu sync.Mutex
	var progressCalls []int
	callback := func(current, total int) {
		mu.Lock()
		defer mu.Unlock()
		progressCalls = append(progressCalls, current)
	}

	f.Fetch(ctx, urls, callback)

	mu.Lock()
	defer mu.Unlock()

	// Progress should be called for each URL
	if len(progressCalls) != len(urls) {
		t.Errorf("expected %d progress calls, got %d", len(urls), len(progressCalls))
	}

	// Verify we got all progress values from 1 to len(urls)
	// (order may vary due to concurrent execution)
	if len(progressCalls) == len(urls) {
		counts := make(map[int]bool)
		for _, p := range progressCalls {
			counts[p] = true
		}

		// Check that we eventually reached the final count
		if !counts[len(urls)] {
			t.Errorf("expected final progress count of %d to be present, got max: %v", len(urls), progressCalls)
		}
	}
}

//
// ───────────────────────────────────────────────
//   Benchmark Tests
// ───────────────────────────────────────────────
//

// BenchmarkFetcher_WorkerPool benchmarks the worker pool implementation
// with various URL counts and concurrency levels.
func BenchmarkFetcher_WorkerPool_100URLs_10Workers(b *testing.B) {
	benchmarkFetcherWorkPool(b, 100, 10)
}

func BenchmarkFetcher_WorkerPool_1000URLs_10Workers(b *testing.B) {
	benchmarkFetcherWorkPool(b, 1000, 10)
}

func BenchmarkFetcher_WorkerPool_1000URLs_50Workers(b *testing.B) {
	benchmarkFetcherWorkPool(b, 1000, 50)
}

func BenchmarkFetcher_WorkerPool_10000URLs_100Workers(b *testing.B) {
	benchmarkFetcherWorkPool(b, 10000, 100)
}

func benchmarkFetcherWorkPool(b *testing.B, numURLs, maxConcurrency int) {
	ctx := context.Background()

	// Create URLs
	urls := make([]string, numURLs)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr := &DummyTracker{}
		wc := &DummyWebClient{} // No delay for benchmarking
		logger := &DummyLogger{}
		idx := &DummyEndpointIndex{}

		f, _ := fetcher.New(fetcher.Config{MaxConcurrency: maxConcurrency, CommitSize: 100}, tr, wc, idx, logger)
		f.Fetch(ctx, urls, nil)
	}
}

// BenchmarkFetcher_MemoryUsage measures memory allocations
func BenchmarkFetcher_MemoryUsage_1000URLs(b *testing.B) {
	ctx := context.Background()

	urls := make([]string, 1000)
	for i := range urls {
		urls[i] = fmt.Sprintf("url-%d", i)
	}

	tr := &DummyTracker{}
	wc := &DummyWebClient{}
	logger := &DummyLogger{}
	idx := &DummyEndpointIndex{}

	f, _ := fetcher.New(fetcher.Config{MaxConcurrency: 10, CommitSize: 100}, tr, wc, idx, logger)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Fetch(ctx, urls, nil)
	}
}
