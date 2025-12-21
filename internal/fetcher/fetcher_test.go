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
	mu      sync.Mutex
	Batches [][]*models.Snapshot
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
	f.Fetch(ctx, urls)

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

	f.Fetch(ctx, urls)

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

	f.Fetch(ctx, []string{"a", "bad", "b"})

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
	f.Fetch(ctx, urls)

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
	f.Fetch(ctx, urls)

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
			f.Fetch(ctx, urls)
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
