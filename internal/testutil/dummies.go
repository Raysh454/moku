// Package testutil provides shared test doubles for use across package tests.
// All dummies implement the corresponding interfaces from the production code,
// allowing injection into components under test without real I/O or side effects.
package testutil

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// ─── Logger ────────────────────────────────────────────────────────────

// DummyLogger implements logging.Logger with in-memory recording.
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

func (l *DummyLogger) With(_ ...logging.Field) logging.Logger { return l }

// ─── WebClient ─────────────────────────────────────────────────────────

// DummyWebClient implements webclient.WebClient.
// By default it returns body "ok:<url>" with status 200.
// Set FailURLs[url] = true to force an error for a specific URL.
type DummyWebClient struct {
	ResponseDelay time.Duration
	FailURLs      map[string]bool
	mu            sync.Mutex
	Requests      []*webclient.Request
}

func (d *DummyWebClient) Do(ctx context.Context, req *webclient.Request) (*webclient.Response, error) {
	if d.ResponseDelay > 0 {
		select {
		case <-time.After(d.ResponseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	d.mu.Lock()
	d.Requests = append(d.Requests, req)
	d.mu.Unlock()

	if d.FailURLs != nil && d.FailURLs[req.URL] {
		return nil, &errString{"dummy fetch fail for " + req.URL}
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

// ─── Tracker ───────────────────────────────────────────────────────────

// DummyTracker implements tracker.Tracker with in-memory recording.
type DummyTracker struct {
	mu      sync.Mutex
	Batches [][]*models.Snapshot
}

func (t *DummyTracker) Commit(_ context.Context, snap *models.Snapshot, _, _ string) (*models.CommitResult, error) {
	return &models.CommitResult{Snapshots: []*models.Snapshot{snap}}, nil
}

func (t *DummyTracker) CommitBatch(_ context.Context, snaps []*models.Snapshot, _, _ string) (*models.CommitResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := append([]*models.Snapshot(nil), snaps...)
	t.Batches = append(t.Batches, cp)
	return &models.CommitResult{Snapshots: cp}, nil
}

func (t *DummyTracker) ScoreAndAttributeVersion(context.Context, *models.CommitResult, time.Duration) error {
	return nil
}

func (t *DummyTracker) SetAssessor(assessor.Assessor) {}

func (t *DummyTracker) GetScoreResultFromSnapshotID(context.Context, string) (*assessor.ScoreResult, error) {
	return nil, nil
}

func (t *DummyTracker) GetScoreResultsFromVersionID(context.Context, string) ([]*assessor.ScoreResult, error) {
	return nil, nil
}

func (t *DummyTracker) GetSecurityDiffOverview(context.Context, string, string) (*assessor.SecurityDiffOverview, error) {
	return nil, nil
}

func (t *DummyTracker) GetSecurityDiff(context.Context, string, string) (*assessor.SecurityDiff, error) {
	return nil, nil
}

func (t *DummyTracker) DiffVersions(context.Context, string, string) (*models.CombinedMultiDiff, error) {
	return nil, nil
}

func (t *DummyTracker) DiffSnapshots(context.Context, string, string) (*models.CombinedFileDiff, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshot(context.Context, string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshots(context.Context, string) ([]*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshotByURL(context.Context, string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetSnapshotByURLAndVersionID(context.Context, string, string) (*models.Snapshot, error) {
	return nil, nil
}

func (t *DummyTracker) GetParentVersionID(context.Context, string) (string, error) { return "", nil }

func (t *DummyTracker) ListVersions(context.Context, int) ([]*models.Version, error) {
	return nil, nil
}

func (t *DummyTracker) Checkout(context.Context, string) error { return nil }

func (t *DummyTracker) HEADExists() (bool, error) { return false, nil }

func (t *DummyTracker) ReadHEAD() (string, error) { return "", nil }

func (t *DummyTracker) DB() *sql.DB { return nil }

func (t *DummyTracker) Close() error { return nil }

// ─── Assessor ──────────────────────────────────────────────────────────

// DummyAssessor implements assessor.Assessor with a preconfigured result.
type DummyAssessor struct {
	Result *assessor.ScoreResult
	Err    error
}

func (d *DummyAssessor) ScoreSnapshot(_ context.Context, _ *models.Snapshot, _ string) (*assessor.ScoreResult, error) {
	if d.Err != nil {
		return nil, d.Err
	}
	if d.Result != nil {
		return d.Result, nil
	}
	return &assessor.ScoreResult{Score: 0.5, Normalized: 50, Confidence: 1, Version: "v-dummy"}, nil
}

func (d *DummyAssessor) Close() error { return nil }

// ─── EndpointIndex ─────────────────────────────────────────────────────

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

func (d *DummyEndpointIndex) AddEndpoints(_ context.Context, _ []string, _ string) ([]string, error) {
	return nil, nil
}

func (d *DummyEndpointIndex) ListEndpoints(_ context.Context, _ string, _ int) ([]indexer.Endpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]indexer.Endpoint(nil), d.ListedEndpoints...), nil
}

func (d *DummyEndpointIndex) MarkPending(context.Context, string) error { return nil }

func (d *DummyEndpointIndex) MarkFetched(context.Context, string, string, time.Time) error {
	return nil
}

func (d *DummyEndpointIndex) MarkFailed(_ context.Context, canonical string, _ string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Failed = append(d.Failed, canonical)
	return d.MarkFailedErr
}

func (d *DummyEndpointIndex) MarkPendingBatch(_ context.Context, canonicals []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.PendingBatches = append(d.PendingBatches, append([]string(nil), canonicals...))
	return d.MarkPendingBatchErr
}

func (d *DummyEndpointIndex) MarkFetchedBatch(_ context.Context, canonicals []string, _ string, _ time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.FetchedBatches = append(d.FetchedBatches, append([]string(nil), canonicals...))
	return d.MarkFetchedBatchErr
}

// ─── Enumerator ────────────────────────────────────────────────────────

// DummyEnumerator implements enumerator.Enumerator.
type DummyEnumerator struct {
	URLs []string
	Err  error
}

func (d *DummyEnumerator) Enumerate(_ context.Context, _ string, cb utils.ProgressCallback) ([]string, error) {
	if d.Err != nil {
		return nil, d.Err
	}
	if cb != nil {
		cb(len(d.URLs), len(d.URLs))
	}
	return d.URLs, nil
}

// ─── helpers ───────────────────────────────────────────────────────────

type errString struct{ s string }

func (e *errString) Error() string { return e.s }
