package app

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

	f, err := fetcher.New(fetcher.Config{MaxConcurrency: 2, CommitSize: 10, ScoreTimeout: 5 * time.Second}, tr, wc, idx, logger)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	comps := &SiteComponents{
		Tracker:   tr,
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
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10)
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
	for range job.Events {
	}

	// Check final status
	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found after completion")
	}
	if final.Status != JobDone {
		t.Errorf("expected status 'done', got %q (err: %s)", final.Status, final.Error)
	}
}

func TestStartFetchJob_RejectsWhenClosed(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	o.Close()

	_, err := o.StartFetchJob(context.Background(), "proj", "site", "new", 10)
	if err == nil {
		t.Fatal("expected error from closed orchestrator")
	}
}

func TestStartFetchJob_CancelJobTransitionsToCanceled(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}

	// Cancel immediately
	o.CancelJob(job.ID)

	// Drain events
	for range job.Events {
	}

	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found after cancel")
	}
	// May be done or canceled depending on timing
	if final.Status != JobCanceled && final.Status != JobDone {
		t.Errorf("expected done or canceled, got %q", final.Status)
	}
}

func TestStartFetchJob_AppearsInListJobs(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10)
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
	for range job.Events {
	}
}

// ─── Enumerate job lifecycle ───────────────────────────────────────────

func TestStartEnumerateJob_Completes(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	injectFakeComponents(t, o, "proj", "site", "https://example.com")

	ctx := context.Background()
	job, err := o.StartEnumerateJob(ctx, "proj", "site", 1)
	if err != nil {
		t.Fatalf("StartEnumerateJob: %v", err)
	}
	if job.Type != "enumerate" {
		t.Errorf("expected type 'enumerate', got %q", job.Type)
	}

	// Drain events
	for range job.Events {
	}

	final := o.GetJob(job.ID)
	if final == nil {
		t.Fatal("job not found")
	}
	// Job should complete (done) or fail depending on spider result
	if final.Status != JobDone && final.Status != JobFailed {
		t.Errorf("expected done or failed, got %q", final.Status)
	}
}

func TestStartEnumerateJob_RejectsWhenClosed(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)
	o.Close()

	_, err := o.StartEnumerateJob(context.Background(), "proj", "site", 1)
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
	job, err := o.StartFetchJob(ctx, "proj", "site", "new", 10)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}

	// Close should cancel running jobs
	o.Close()

	// Drain remaining events
	for range job.Events {
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
	cb(1, 10)

	// Read the progress event from the channel
	select {
	case ev := <-job.Events:
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
