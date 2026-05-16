package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/api"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/testutil"

	_ "modernc.org/sqlite"
)

// newTestOrchestrator creates an Orchestrator with an in-memory registry + TempDir.
// Returns the orchestrator, the registry, and a cleanup func.
func newTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := &testutil.DummyLogger{}
	reg, err := registry.NewRegistry(db, dir, logger)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	cfg := DefaultConfig()
	cfg.StorageRoot = dir
	cfg.JobRetentionTime = 5 * time.Second

	orch := NewOrchestrator(cfg, reg, logger)
	t.Cleanup(func() { orch.Close() })
	return orch
}

// injectFakeComponents creates a project + website in the registry,
// then injects dummy-backed SiteComponents into the orchestrator cache.
func injectFakeComponents(t *testing.T, o *Orchestrator, projectSlug, websiteSlug, origin string) {
	t.Helper()
	ctx := context.Background()

	_, err := o.CreateProject(ctx, projectSlug, projectSlug, "test project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	web, err := o.CreateWebsite(ctx, projectSlug, websiteSlug, origin)
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	logger := &testutil.DummyLogger{}
	tr := &testutil.DummyTracker{}
	wc := &testutil.DummyWebClient{}
	idx := &testutil.DummyEndpointIndex{}
	an := &testutil.DummyAnalyzer{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 2, CommitSize: 10, ScoreTimeout: 5 * time.Second}, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	comps := &SiteComponents{
		Tracker:   tr,
		Index:     idx,
		Fetcher:   f,
		WebClient: wc,
		Analyzer:  an,
	}

	o.siteCompMutex.Lock()
	if o.siteComponentsCache == nil {
		o.siteComponentsCache = make(map[string]*SiteComponents)
	}
	o.siteComponentsCache[web.ID] = comps
	o.siteCompMutex.Unlock()
}

// ─── Construction ──────────────────────────────────────────────────────

func TestNewOrchestrator_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	if o == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}

func TestNewOrchestrator_DefaultConfig(t *testing.T) {
	t.Parallel()
	logger := &testutil.DummyLogger{}
	o := NewOrchestrator(nil, nil, logger)
	if o.cfg == nil {
		t.Fatal("expected default config when nil passed")
	}
}

// ─── CRUD delegates ────────────────────────────────────────────────────

func TestOrchestrator_CreateAndListProjects(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	ctx := context.Background()

	p, err := o.CreateProject(ctx, "test-proj", "Test Project", "desc")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Slug != "test-proj" {
		t.Errorf("expected slug 'test-proj', got %q", p.Slug)
	}

	projects, err := o.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
}

func TestOrchestrator_CreateAndListWebsites(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	ctx := context.Background()

	_, err := o.CreateProject(ctx, "proj", "Proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	web, err := o.CreateWebsite(ctx, "proj", "site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if web.Origin != "https://example.com" {
		t.Errorf("unexpected origin: %q", web.Origin)
	}

	websites, err := o.ListWebsites(ctx, "proj")
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(websites) != 1 {
		t.Fatalf("expected 1 website, got %d", len(websites))
	}
}

// ─── Job management ────────────────────────────────────────────────────

func TestGetJob_ReturnsNilForUnknown(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	if j := o.GetJob("nonexistent"); j != nil {
		t.Errorf("expected nil for unknown job, got %+v", j)
	}
}

func TestListJobs_EmptyInitially(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	jobs := o.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestCancelJob_NoOpForUnknown(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	// Should not panic
	o.CancelJob("does-not-exist")
}

// ─── Fetch job lifecycle ───────────────────────────────────────────────

func TestStartFetchJob_TransitionsToRunningThenDone(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10, nil)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Type != "fetch" {
		t.Errorf("expected type 'fetch', got %q", job.Type)
	}

	// Wait for job to finish by draining events
	events := o.Subscribe(ctx)
	for ev := range events {
		if ev.JobID == job.ID && (ev.Status == JobDone || ev.Status == JobFailed || ev.Status == JobCanceled) {
			break
		}
	}

	// Check final status
	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found after completion")
	} else {
		if final.Status != JobDone {
			t.Errorf("expected status 'done', got %q (err: %s)", final.Status, final.Error)
		}
	}
}

func TestOrchestrator_DeleteWebsite_ClosesCachedComponents(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	ctx := context.Background()

	if _, err := o.CreateProject(ctx, "proj", "Proj", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	web, err := o.CreateWebsite(ctx, "proj", "site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	closed := false
	tr := &testutil.DummyTracker{CloseFunc: func() error {
		closed = true
		return nil
	}}
	wc := &testutil.DummyWebClient{}
	idx := &testutil.DummyEndpointIndex{}
	an := &testutil.DummyAnalyzer{}
	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 2, CommitSize: 10, ScoreTimeout: 5 * time.Second}, tr, wc, idx, &testutil.DummyLogger{})
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	o.siteCompMutex.Lock()
	if o.siteComponentsCache == nil {
		o.siteComponentsCache = make(map[string]*SiteComponents)
	}
	o.siteComponentsCache[web.ID] = &SiteComponents{Tracker: tr, Index: idx, Fetcher: f, WebClient: wc, Analyzer: an}
	o.siteCompMutex.Unlock()

	if err := o.DeleteWebsite(ctx, "proj", "site"); err != nil {
		t.Fatalf("DeleteWebsite: %v", err)
	}
	if !closed {
		t.Fatal("expected cached tracker to be closed before deletion")
	}
	o.siteCompMutex.Lock()
	_, ok := o.siteComponentsCache[web.ID]
	o.siteCompMutex.Unlock()
	if ok {
		t.Fatal("expected cached site components to be evicted after deletion")
	}
	if _, err := os.Stat(web.StoragePath); !os.IsNotExist(err) {
		t.Fatalf("expected website directory to be removed, stat err = %v", err)
	}
}

// TestOrchestrator_DeleteWebsite_CancelsInFlightScanJob proves that an
// in-flight scan job is actually cancelled (and waited on) before the
// orchestrator evicts the site. Before the cancellation-filter fix this
// test would deadlock or timeout because `cancelWebsiteJobs` filtered on
// the project UUID while `Job.Project` holds the URL slug, so the cancel
// loop matched nothing and `waitForWebsiteJobs` returned immediately even
// while the goroutine was still running.
func TestOrchestrator_DeleteWebsite_CancelsInFlightScanJob(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "http://example.com")

	web, err := o.registry.GetWebsiteBySlug(context.Background(), "proj", "site")
	if err != nil {
		t.Fatalf("GetWebsiteBySlug: %v", err)
	}

	scanStarted := make(chan struct{})
	scanExited := make(chan struct{})
	o.siteCompMutex.Lock()
	comps := o.siteComponentsCache[web.ID]
	o.siteCompMutex.Unlock()
	comps.Analyzer = &testutil.DummyAnalyzer{
		ScanAndWaitFunc: func(ctx context.Context, _ *analyzer.ScanRequest, _ analyzer.PollOptions) (*analyzer.ScanResult, error) {
			close(scanStarted)
			<-ctx.Done()
			close(scanExited)
			return nil, ctx.Err()
		},
	}

	job, err := o.StartScanJob(context.Background(), "proj", "site", &analyzer.ScanRequest{URL: "http://example.com/"})
	if err != nil {
		t.Fatalf("StartScanJob: %v", err)
	}

	select {
	case <-scanStarted:
	case <-time.After(time.Second):
		t.Fatal("scan goroutine did not start within 1s")
	}

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- o.DeleteWebsite(context.Background(), "proj", "site") }()

	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("DeleteWebsite: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DeleteWebsite did not return within 5s — cancellation likely not firing")
	}

	select {
	case <-scanExited:
	case <-time.After(time.Second):
		t.Fatal("scan goroutine did not observe cancellation")
	}

	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("scan job vanished from orchestrator")
	}
	if final.Status != JobCanceled {
		t.Errorf("scan job status = %q, want %q", final.Status, JobCanceled)
	}
}

func TestStartFetchJob_RejectsWhenClosed(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	o.Close()

	_, err := o.StartFetchJob(context.Background(), "proj", "site", "new", 10, nil)
	if err == nil {
		t.Fatal("expected error from closed orchestrator")
	}
}

func TestStartFetchJob_CancelJobTransitionsToCanceled(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10, nil)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}

	// Cancel immediately
	o.CancelJob(job.ID)

	// Drain events
	events := o.Subscribe(ctx)
	for ev := range events {
		if ev.JobID == job.ID && (ev.Status == JobDone || ev.Status == JobFailed || ev.Status == JobCanceled) {
			break
		}
	}

	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found after cancel")
	} else {
		// May be done or canceled depending on timing
		if final.Status != JobCanceled && final.Status != JobDone {
			t.Errorf("expected done or canceled, got %q", final.Status)
		}
	}
}

func TestStartFetchJob_AppearsInListJobs(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10, nil)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}

	jobs := o.ListJobs()
	found := false
	for _, j := range jobs {
		if j.ID == job.ID {
			found = true
		}
	}
	if !found {
		t.Error("started job not found in ListJobs")
	}

	// Drain events so job finishes
	events := o.Subscribe(ctx)
	for ev := range events {
		if ev.JobID == job.ID && (ev.Status == JobDone || ev.Status == JobFailed || ev.Status == JobCanceled) {
			break
		}
	}
}

// ─── Enumerate job lifecycle ───────────────────────────────────────────

func TestStartEnumerateJob_Completes(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartEnumerateJob(ctx, "proj", "site", api.EnumerationConfig{Spider: &api.SpiderConfig{MaxDepth: 1}})
	if err != nil {
		t.Fatalf("StartEnumerateJob: %v", err)
	}
	if job.Type != "enumerate" {
		t.Errorf("expected type 'enumerate', got %q", job.Type)
	}

	// Drain events
	events := o.Subscribe(ctx)
	for ev := range events {
		if ev.JobID == job.ID && (ev.Status == JobDone || ev.Status == JobFailed || ev.Status == JobCanceled) {
			break
		}
	}

	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found")
	} else {
		// Job should complete (done) or fail depending on spider result
		if final.Status != JobDone && final.Status != JobFailed {
			t.Errorf("expected done or failed, got %q", final.Status)
		}
	}
}

func TestStartEnumerateJob_RejectsWhenClosed(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	o.Close()

	_, err := o.StartEnumerateJob(context.Background(), "proj", "site", api.EnumerationConfig{})
	if err == nil {
		t.Fatal("expected error from closed orchestrator")
	}
}

// ─── Close ─────────────────────────────────────────────────────────────

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	// Should not panic when called multiple times
	o.Close()
	o.Close()
}

func TestClose_CancelsRunningJobs(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10, nil)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}

	// Close should cancel running jobs
	o.Close()

	// Drain remaining events
	events := o.Subscribe(ctx)
	for ev := range events {
		if ev.JobID == job.ID && (ev.Status == JobDone || ev.Status == JobFailed || ev.Status == JobCanceled) {
			break
		}
	}
}

// ─── Progress callback ─────────────────────────────────────────────────

func TestProgressCallback_EmitsProgressEvents(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	o.ensureJobMaps()

	job := o.newJob("fetch", "proj", "site")
	o.setJob(job)

	cb := o.progressCallback(job.ID)
	// Subscribe before emitting
	events := o.Subscribe(context.Background())
	cb(1, 0, 10)

	// Read the progress event from the channel
	select {
	case ev := <-events:
		if ev.Type != JobEventProgress {
			t.Errorf("expected progress event, got %q", ev.Type)
		}
		if ev.Processed != 1 || ev.Total != 10 {
			t.Errorf("expected 1/10, got %d/%d", ev.Processed, ev.Total)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for progress event")
	}
}

// ─── GetEndpointDetails with version params ────────────────────────────

func TestGetEndpointDetails_NoVersionParams_BackwardCompatible(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()

	// Should work without version params (backward compatible)
	_, err := o.GetEndpointDetails(ctx, "proj", "site", "https://example.com/page", "", "")
	if err == nil {
		// DummyTracker will return error because no snapshot exists, but no panic/crash
		t.Log("expected error from dummy tracker, got none")
	}
}

func TestGetEndpointDetails_OnlyBaseVersion_ReturnsError(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()

	// Should return error when only baseVersionID is provided
	_, err := o.GetEndpointDetails(ctx, "proj", "site", "https://example.com/page", "v1", "")
	if err == nil {
		t.Error("expected error when only base_version_id provided")
	}
	if err != nil && err.Error() != "both base_version_id and head_version_id must be provided together" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetEndpointDetails_OnlyHeadVersion_ReturnsError(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()

	// Should return error when only headVersionID is provided
	_, err := o.GetEndpointDetails(ctx, "proj", "site", "https://example.com/page", "", "v2")
	if err == nil {
		t.Error("expected error when only head_version_id provided")
	}
	if err != nil && err.Error() != "both base_version_id and head_version_id must be provided together" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetEndpointDetails_BothVersions_CallsTrackerCorrectly(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	ctx := context.Background()

	_, err := o.CreateProject(ctx, "proj", "Proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	web, err := o.CreateWebsite(ctx, "proj", "site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	// Create a spy tracker to track method calls
	logger := &testutil.DummyLogger{}
	spyTracker := &testutil.SpyTracker{DummyTracker: &testutil.DummyTracker{}}
	idx := &testutil.DummyEndpointIndex{}
	wc := &testutil.DummyWebClient{}

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 2, CommitSize: 10, ScoreTimeout: 5 * time.Second}, spyTracker, wc, idx, logger)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	comps := &SiteComponents{
		Tracker:   spyTracker,
		Index:     idx,
		Fetcher:   f,
		WebClient: wc,
	}

	o.siteCompMutex.Lock()
	if o.siteComponentsCache == nil {
		o.siteComponentsCache = make(map[string]*SiteComponents)
	}
	o.siteComponentsCache[web.ID] = comps
	o.siteCompMutex.Unlock()

	// Call with both version IDs
	_, err = o.GetEndpointDetails(ctx, "proj", "site", "https://example.com/page", "v1", "v2")

	// Should call GetSnapshotByURLAndVersionID for both versions
	// Error is expected since spy tracker returns nil snapshots, but we verify the call was made
	if err == nil {
		t.Error("expected error from spy tracker (no snapshots)")
	}

	if !spyTracker.GetSnapshotByURLAndVersionIDCalled {
		t.Error("expected GetSnapshotByURLAndVersionID to be called")
	}
}

func TestListVersions_DelegatesToTracker(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()

	// Should not panic, will return empty list from DummyTracker
	versions, err := o.ListVersions(ctx, "proj", "site", 10)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}

	// DummyTracker returns empty list
	if len(versions) != 0 {
		t.Errorf("expected 0 versions from DummyTracker, got %d", len(versions))
	}
}

func TestListVersions_InvalidProject_ReturnsError(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	ctx := context.Background()

	_, err := o.ListVersions(ctx, "nonexistent", "site", 10)
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestListVersions_InvalidWebsite_ReturnsError(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	ctx := context.Background()

	_, err := o.CreateProject(ctx, "proj", "Proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	_, err = o.ListVersions(ctx, "proj", "nonexistent", 10)
	if err == nil {
		t.Error("expected error for nonexistent website")
	}
}

// ─── Analyzer plugin wiring ────────────────────────────────────────────

// TestNewSiteComponents_HasAnalyzer verifies the real NewSiteComponents
// constructs a non-nil analyzer for the site, proving the plugin plumbing
// from app.Config through analyzer.NewAnalyzer works end-to-end.
func TestNewSiteComponents_HasAnalyzer(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	ctx := context.Background()

	if _, err := o.CreateProject(ctx, "scan-proj", "Scan Proj", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	web, err := o.CreateWebsite(ctx, "scan-proj", "scan-site", "http://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	comps, err := NewSiteComponents(ctx, o.cfg, *web, &testutil.DummyLogger{})
	if err != nil {
		t.Fatalf("NewSiteComponents: %v", err)
	}
	t.Cleanup(func() { _ = comps.Close() })

	if comps.Analyzer == nil {
		t.Fatal("SiteComponents.Analyzer is nil; want non-nil from analyzer.NewAnalyzer")
	}
	if got := string(comps.Analyzer.Name()); got != "moku" {
		t.Errorf("Analyzer.Name() = %q, want %q (default backend)", got, "moku")
	}
}

// TestOrchestrator_GetAnalyzer_ReturnsSiteAnalyzer verifies the orchestrator
// exposes a site's analyzer for HTTP handlers needing Capabilities/Health.
func TestOrchestrator_GetAnalyzer_ReturnsSiteAnalyzer(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "http://example.com")

	a, err := o.GetAnalyzer(context.Background(), "proj", "site")
	if err != nil {
		t.Fatalf("GetAnalyzer: %v", err)
	}
	if a == nil {
		t.Fatal("GetAnalyzer returned nil analyzer with nil error")
	}
}

// TestOrchestrator_StartScanJob_NilRequestErrors asserts input validation.
func TestOrchestrator_StartScanJob_NilRequestErrors(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	if _, err := o.StartScanJob(context.Background(), "proj", "site", nil); err == nil {
		t.Error("expected error for nil scan request")
	}
}

// TestOrchestrator_StartScanJob_EmptyURLErrors asserts input validation.
func TestOrchestrator_StartScanJob_EmptyURLErrors(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	if _, err := o.StartScanJob(context.Background(), "proj", "site", &analyzer.ScanRequest{URL: ""}); err == nil {
		t.Error("expected error for empty scan URL")
	}
}

// TestOrchestrator_StartScanJob_CompletesAndStoresResult drives the scan-job
// goroutine through to JobDone using the DummyAnalyzer and asserts that the
// terminal Job carries a populated ScanResult.
func TestOrchestrator_StartScanJob_CompletesAndStoresResult(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "http://example.com")

	job, err := o.StartScanJob(context.Background(), "proj", "site", &analyzer.ScanRequest{URL: "http://example.com/"})
	if err != nil {
		t.Fatalf("StartScanJob: %v", err)
	}
	if job.Type != "scan" {
		t.Errorf("job.Type = %q, want %q", job.Type, "scan")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored := o.GetJob(job.ID)
		if stored != nil && stored.Status == JobDone {
			if stored.ScanResult == nil {
				t.Fatal("stored.ScanResult is nil; want non-nil")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan job did not reach JobDone within deadline")
}
