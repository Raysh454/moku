package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"strings"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/api"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

type JobEventType string

const (
	JobEventStatus   JobEventType = "status"
	JobEventProgress JobEventType = "progress"
	JobEventResult   JobEventType = "result"
)

type JobEvent struct {
	JobID     string       `json:"job_id"`
	Type      JobEventType `json:"type"`
	Status    JobStatus    `json:"status,omitempty"`
	Error     string       `json:"error,omitempty"`
	Processed int          `json:"processed,omitempty"`
	Total     int          `json:"total,omitempty"`
}

type JobStatus string

const (
	JobPending  JobStatus = "pending"
	JobRunning  JobStatus = "running"
	JobDone     JobStatus = "done"
	JobFailed   JobStatus = "failed"
	JobCanceled JobStatus = "canceled"
)

type Job struct {
	ID               string                         `json:"id"`
	Type             string                         `json:"type"`
	Project          string                         `json:"project"`
	Website          string                         `json:"website"`
	Status           JobStatus                      `json:"status"`
	Error            string                         `json:"error,omitempty"`
	StartedAt        time.Time                      `json:"started_at"`
	EndedAt          time.Time                      `json:"ended_at"`
	Events           chan JobEvent                  `json:"-"`
	SecurityOverview *assessor.SecurityDiffOverview `json:"security_overview,omitempty"`
	EnumeratedURLs   []string                       `json:"enumerated_urls,omitempty"`
	// ScanResult is populated when Type == "scan" and the analyzer pipeline
	// completed. It carries the unified industry-shaped Findings list.
	ScanResult *analyzer.ScanResult `json:"scan_result,omitempty"`
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	copy := *job
	return &copy
}

type Orchestrator struct {
	cfg      *Config
	registry *registry.Registry
	logger   logging.Logger

	siteCompMutex       sync.Mutex
	siteComponentsCache map[string]*SiteComponents

	jobsMu           sync.Mutex
	jobs             map[string]*Job
	jobCancels       map[string]context.CancelFunc
	jobRetentionTime time.Duration

	closedMu sync.Mutex
	closed   bool
}

func NewOrchestrator(cfg *Config, reg *registry.Registry, logger logging.Logger) *Orchestrator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Orchestrator{
		cfg:              cfg,
		registry:         reg,
		logger:           logger,
		jobRetentionTime: cfg.JobRetentionTime,
	}
}

// SeedDefaultFiltersForAllWebsites seeds default filter rules for websites that don't have any.
// This ensures backwards compatibility for websites created before the seeding feature.
func (o *Orchestrator) SeedDefaultFiltersForAllWebsites(ctx context.Context) error {
	return o.registry.SeedDefaultsForAllWebsites(ctx)
}

// Registry returns the underlying registry.
func (o *Orchestrator) Registry() *registry.Registry {
	return o.registry
}

func (o *Orchestrator) ensureJobMaps() {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	if o.jobs == nil {
		o.jobs = make(map[string]*Job)
	}
	if o.jobCancels == nil {
		o.jobCancels = make(map[string]context.CancelFunc)
	}
}

func (o *Orchestrator) emitJobEvent(jobID string, ev JobEvent) {
	o.jobsMu.Lock()
	job, ok := o.jobs[jobID]
	o.jobsMu.Unlock()
	if !ok || job == nil || job.Events == nil {
		return
	}

	select {
	case job.Events <- ev:
	default:
	}
}

func (o *Orchestrator) setJob(job *Job) {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	if o.jobs == nil {
		o.jobs = make(map[string]*Job)
	}
	o.jobs[job.ID] = job
}

func (o *Orchestrator) cleanupFinishedJobsLocked() {
	if o.jobRetentionTime <= 0 {
		return
	}

	now := time.Now().UTC()
	for id, job := range o.jobs {
		if job == nil {
			delete(o.jobs, id)
			continue
		}

		if job.Status != JobDone && job.Status != JobFailed && job.Status != JobCanceled {
			continue
		}
		if job.EndedAt.IsZero() {
			continue
		}

		if now.Sub(job.EndedAt) > o.jobRetentionTime {
			delete(o.jobs, id)
		}
	}
}

func (o *Orchestrator) setCancel(jobID string, cancel context.CancelFunc) {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	if o.jobCancels == nil {
		o.jobCancels = make(map[string]context.CancelFunc)
	}
	o.jobCancels[jobID] = cancel
}

func (o *Orchestrator) deleteCancel(jobID string) {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	delete(o.jobCancels, jobID)
}

func (o *Orchestrator) getCancel(jobID string) context.CancelFunc {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	return o.jobCancels[jobID]
}

func (o *Orchestrator) newJob(jobType, project, site string) *Job {
	return &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Project:   project,
		Website:   site,
		Status:    JobPending,
		StartedAt: time.Now().UTC(),
		Events:    make(chan JobEvent, 16),
	}
}

func (o *Orchestrator) finishJob(jobID string) {
	o.jobsMu.Lock()
	if j, ok := o.jobs[jobID]; ok {
		j.EndedAt = time.Now().UTC()
	}

	o.cleanupFinishedJobsLocked()

	events := o.getJobEvents(jobID)
	o.jobsMu.Unlock()

	o.deleteCancel(jobID)

	if events != nil {
		close(events)
	}
}

func (o *Orchestrator) getJobEvents(jobID string) chan JobEvent {
	var events chan JobEvent
	if j, ok := o.jobs[jobID]; ok && j != nil {
		events = j.Events
	}
	return events
}

func (o *Orchestrator) setJobStatus(jobID string, status JobStatus, err error) {
	o.jobsMu.Lock()
	if j, ok := o.jobs[jobID]; ok {
		j.Status = status
		if err != nil {
			j.Error = err.Error()
		}
	}
	o.jobsMu.Unlock()

	ev := JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: status,
	}
	if err != nil {
		ev.Error = err.Error()
	}
	o.emitJobEvent(jobID, ev)
}

func (o *Orchestrator) setJobResult(jobID string, overview *assessor.SecurityDiffOverview, urls []string) {
	o.jobsMu.Lock()
	if j, ok := o.jobs[jobID]; ok {
		j.Status = JobDone
		j.SecurityOverview = overview
		j.EnumeratedURLs = urls
	}
	o.jobsMu.Unlock()

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventResult,
		Status: JobDone,
	})
}

// setScanJobResult records the analyzer's ScanResult on the Job and emits a
// result event. Separate from setJobResult because scan jobs carry a
// different payload type than fetch/enumerate jobs.
func (o *Orchestrator) setScanJobResult(jobID string, scan *analyzer.ScanResult) {
	o.jobsMu.Lock()
	if j, ok := o.jobs[jobID]; ok {
		j.Status = JobDone
		j.ScanResult = scan
	}
	o.jobsMu.Unlock()

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventResult,
		Status: JobDone,
	})
}

func (o *Orchestrator) progressCallback(jobID string) utils.ProgressCallback {
	return func(processed, total int) {
		o.emitJobEvent(jobID, JobEvent{
			JobID:     jobID,
			Type:      JobEventProgress,
			Processed: processed,
			Total:     total,
		})
	}
}

func (o *Orchestrator) StartFetchJob(ctx context.Context, project, site, status string, limit int, cfg *api.FetchConfig) (*Job, error) {
	o.logger.Info("orchestrator: Starting fetch job", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
	o.closedMu.Lock()
	closed := o.closed
	o.closedMu.Unlock()
	if closed {
		return nil, fmt.Errorf("orchestrator is closed")
	}
	o.ensureJobMaps()

	job := o.newJob("fetch", project, site)
	jobID := job.ID

	o.setJob(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.setCancel(jobID, cancel)

	// Emit initial pending event
	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.finishJob(jobID)
		o.setJobStatus(jobID, JobRunning, nil)

		cb := o.progressCallback(jobID)
		o.logger.Info("Starting fetch job", logging.Field{Key: "job_id", Value: jobID}, logging.Field{Key: "website", Value: site}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
		overview, err := o.FetchWebsiteEndpoints(jobCtx, project, site, status, limit, cfg, cb)
		if err != nil {
			o.logger.Error("Orchestrator (StartFetchJob): Fetch job failed", logging.Field{Key: "job_id", Value: jobID}, logging.Field{Key: "error", Value: err.Error()})
			select {
			case <-jobCtx.Done():
				o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
			default:
				o.setJobStatus(jobID, JobFailed, err)
			}
			return
		}

		select {
		case <-jobCtx.Done():
			o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
		default:
			o.setJobResult(jobID, overview, nil)
		}
	}()

	return jobSnapshot, nil
}

func (o *Orchestrator) StartEnumerateJob(ctx context.Context, project, site string, cfg api.EnumerationConfig) (*Job, error) {
	o.closedMu.Lock()
	closed := o.closed
	o.closedMu.Unlock()
	if closed {
		return nil, fmt.Errorf("orchestrator is closed")
	}
	o.ensureJobMaps()

	job := o.newJob("enumerate", project, site)
	jobID := job.ID

	o.setJob(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.setCancel(jobID, cancel)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.finishJob(jobID)
		o.setJobStatus(jobID, JobRunning, nil)

		cb := o.progressCallback(jobID)
		urls, err := o.EnumerateWebsite(jobCtx, project, site, cfg, cb)
		if err != nil {
			select {
			case <-jobCtx.Done():
				o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
			default:
				o.setJobStatus(jobID, JobFailed, err)
			}
			return
		}

		select {
		case <-jobCtx.Done():
			o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
		default:
			o.setJobResult(jobID, nil, urls)
		}
	}()

	return jobSnapshot, nil
}

// StartScanJob kicks off a vulnerability scan for project/site using whatever
// analyzer backend the site was configured with. Mirrors StartFetchJob /
// StartEnumerateJob: returns immediately with a Job snapshot and runs the
// scan in a goroutine, emitting JobEvents as state transitions.
func (o *Orchestrator) StartScanJob(ctx context.Context, projectIdentifier, websiteSlug string, req *analyzer.ScanRequest) (*Job, error) {
	if req == nil {
		return nil, fmt.Errorf("StartScanJob: nil request")
	}
	if req.URL == "" {
		return nil, fmt.Errorf("StartScanJob: empty URL")
	}

	o.closedMu.Lock()
	closed := o.closed
	o.closedMu.Unlock()
	if closed {
		return nil, fmt.Errorf("orchestrator is closed")
	}
	o.ensureJobMaps()

	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	if comps.Analyzer == nil {
		return nil, fmt.Errorf("StartScanJob: site components have no analyzer")
	}

	job := o.newJob("scan", projectIdentifier, websiteSlug)
	jobID := job.ID
	o.setJob(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.setCancel(jobID, cancel)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.finishJob(jobID)
		o.setJobStatus(jobID, JobRunning, nil)

		result, err := comps.Analyzer.ScanAndWait(jobCtx, req, o.cfg.AnalyzerCfg.DefaultPoll)
		if err != nil {
			select {
			case <-jobCtx.Done():
				o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
			default:
				o.setJobStatus(jobID, JobFailed, err)
			}
			return
		}

		select {
		case <-jobCtx.Done():
			o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
		default:
			o.setScanJobResult(jobID, result)
		}
	}()

	return jobSnapshot, nil
}

// GetAnalyzer returns the analyzer.Analyzer instance for project/site. Used
// by HTTP handlers that need to expose Capabilities / Health without going
// through the job pipeline.
func (o *Orchestrator) GetAnalyzer(ctx context.Context, projectIdentifier, websiteSlug string) (analyzer.Analyzer, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	if comps.Analyzer == nil {
		return nil, fmt.Errorf("no analyzer configured for site %s/%s", projectIdentifier, websiteSlug)
	}
	return comps.Analyzer, nil
}

func (o *Orchestrator) CancelJob(jobID string) {
	cancel := o.getCancel(jobID)
	if cancel != nil {
		cancel()
	}
}

func (o *Orchestrator) GetJob(jobID string) *Job {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()
	j, ok := o.jobs[jobID]
	if !ok {
		return nil
	}
	return cloneJob(j)
}

func (o *Orchestrator) ListJobs() []*Job {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()

	jobs := make([]*Job, 0, len(o.jobs))
	for _, j := range o.jobs {
		if j != nil {
			jobs = append(jobs, cloneJob(j))
		}
	}
	return jobs
}

func (o *Orchestrator) siteComponentsFor(ctx context.Context, web *registry.Website) (*SiteComponents, error) {
	o.siteCompMutex.Lock()
	if o.siteComponentsCache == nil {
		o.siteComponentsCache = make(map[string]*SiteComponents)
	}
	if comps, ok := o.siteComponentsCache[web.ID]; ok {
		o.siteCompMutex.Unlock()
		return comps, nil
	}
	o.siteCompMutex.Unlock()

	comps, err := NewSiteComponents(ctx, o.cfg, *web, o.logger)
	if err != nil {
		return nil, err
	}

	o.siteCompMutex.Lock()
	o.siteComponentsCache[web.ID] = comps
	o.siteCompMutex.Unlock()

	return comps, nil
}

func (o *Orchestrator) CreateProject(ctx context.Context, slug, name, description string) (*registry.Project, error) {
	return o.registry.CreateProject(ctx, slug, name, description)
}

func (o *Orchestrator) ListProjects(ctx context.Context) ([]registry.Project, error) {
	return o.registry.ListProjects(ctx)
}

func (o *Orchestrator) CreateWebsite(ctx context.Context, projectIdentifier, slug, origin string) (*registry.Website, error) {
	return o.registry.CreateWebsite(ctx, projectIdentifier, slug, origin)
}

func (o *Orchestrator) ListWebsites(ctx context.Context, projectIdentifier string) ([]registry.Website, error) {
	return o.registry.ListWebsites(ctx, projectIdentifier)
}

// GetWebsiteIndexer returns the indexer for a website.
func (o *Orchestrator) GetWebsiteIndexer(ctx context.Context, projectIdentifier, websiteSlug string) (*indexer.Index, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	// The Index field is of type EndpointIndex interface, but the actual
	// implementation is *indexer.Index which has additional methods.
	idx, ok := comps.Index.(*indexer.Index)
	if !ok {
		return nil, fmt.Errorf("indexer is not of expected type")
	}
	return idx, nil
}

func (o *Orchestrator) AddWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug string, rawURLs []string, source string) ([]string, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	return comps.Index.AddEndpoints(ctx, rawURLs, source)
}

func (o *Orchestrator) EnumerateWebsite(ctx context.Context, projectIdentifier, websiteSlug string, cfg api.EnumerationConfig, cb utils.ProgressCallback) ([]string, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}

	enum, source := o.buildEnumerator(cfg, comps.WebClient)
	targets, err := enum.Enumerate(ctx, web.Origin, cb)
	if err != nil {
		return nil, err
	}

	_, err = comps.Index.AddEndpoints(ctx, targets, source)
	if err != nil {
		return nil, err
	}
	return targets, nil
}

func (o *Orchestrator) buildEnumerator(cfg api.EnumerationConfig, wc webclient.WebClient) (enumerator.Enumerator, string) {
	var enumerators []enumerator.Enumerator
	var methods []string

	if cfg.Spider != nil {
		maxDepth := cfg.Spider.MaxDepth
		if maxDepth == 0 {
			maxDepth = 4 // default
		}
		enumerators = append(enumerators, enumerator.NewSpider(maxDepth, wc, o.logger))
		methods = append(methods, "spider")
	}

	if cfg.Sitemap != nil {
		enumerators = append(enumerators, enumerator.NewSitemap(wc, o.logger))
		methods = append(methods, "sitemap")
	}

	if cfg.Robots != nil {
		enumerators = append(enumerators, enumerator.NewRobots(wc, o.logger))
		methods = append(methods, "robots")
	}

	if cfg.Wayback != nil {
		wbCfg := &enumerator.WaybackConfig{
			UseWaybackMachine: cfg.Wayback.UseWaybackMachine,
			UseCommonCrawl:    cfg.Wayback.UseCommonCrawl,
		}
		enumerators = append(enumerators, enumerator.NewWaybackWithConfig(wc, o.logger, wbCfg))
		methods = append(methods, "wayback")
	}

	// Default to spider if nothing specified
	if len(enumerators) == 0 {
		enumerators = append(enumerators, enumerator.NewSpider(4, wc, o.logger))
		methods = append(methods, "spider")
	}

	source := strings.Join(methods, "+") + "-enumerator"
	if len(enumerators) == 1 {
		return enumerators[0], source
	}
	return enumerator.NewComposite(enumerators, o.logger), source
}

func (o *Orchestrator) ListWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int) ([]indexer.Endpoint, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	o.logger.Info("Listing endpoints", logging.Field{Key: "website_id", Value: web.ID}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
	return comps.Index.ListEndpoints(ctx, status, limit)
}

func (o *Orchestrator) ListVersions(ctx context.Context, projectIdentifier, websiteSlug string, limit int) ([]*models.Version, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	o.logger.Info("Listing versions", logging.Field{Key: "website_id", Value: web.ID}, logging.Field{Key: "limit", Value: limit})
	return comps.Tracker.ListVersions(ctx, limit)
}

func (o *Orchestrator) FetchWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int, cfg *api.FetchConfig, cb utils.ProgressCallback) (*assessor.SecurityDiffOverview, error) {
	if status == "" {
		status = "*"
	}

	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}

	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}

	filterCfg, err := o.registry.LoadFilterConfig(ctx, web.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("loading filter config: %w", err)
	}
	fetchOpts := &fetcher.FetchOptions{
		FilterConfig:      filterCfg,
		FilterStatusCodes: len(filterCfg.SkipStatusCodes) > 0,
	}

	// Use custom fetcher if config provided
	fetcher := comps.Fetcher
	if cfg != nil && cfg.Concurrency > 0 {
		// Create custom fetcher config
		fetcherCfg := o.cfg.FetcherCfg
		fetcherCfg.MaxConcurrency = cfg.Concurrency

		customFetcher, err := o.createCustomFetcher(fetcherCfg, comps)
		if err != nil {
			return nil, fmt.Errorf("creating custom fetcher: %w", err)
		}
		fetcher = customFetcher
	}

	headExists, err := comps.Tracker.HEADExists()
	if err != nil {
		return nil, err
	}

	if !headExists {
		if err := fetcher.FetchFromIndexWithOptions(ctx, status, limit, fetchOpts, cb); err != nil {
			return nil, err
		}
		return nil, nil
	}

	prevHeadID, err := comps.Tracker.ReadHEAD()
	if err != nil {
		return nil, err
	}

	o.logger.Info("Starting fetch of website endpoints", logging.Field{Key: "website_id", Value: web.ID}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit}, logging.Field{Key: "previous_head_id", Value: prevHeadID})
	if err := fetcher.FetchFromIndexWithOptions(ctx, status, limit, fetchOpts, cb); err != nil {
		return nil, err
	}

	newHeadID, err := comps.Tracker.ReadHEAD()
	if err != nil {
		return nil, err
	}

	if prevHeadID == "" || prevHeadID == newHeadID {
		return nil, nil
	}

	return comps.Tracker.GetSecurityDiffOverview(ctx, prevHeadID, newHeadID)
}

// createCustomFetcher creates a new fetcher with custom configuration
func (o *Orchestrator) createCustomFetcher(fetcherCfg fetcher.Config, comps *SiteComponents) (*fetcher.Fetcher, error) {
	return fetcher.New(
		fetcherCfg,
		comps.Tracker,
		comps.WebClient,
		comps.Index,
		o.logger,
	)
}

type EndpointDetails struct {
	Snapshot     *models.Snapshot         `json:"snapshot"`
	ScoreResult  *assessor.ScoreResult    `json:"score_result,omitempty"`
	SecurityDiff *assessor.SecurityDiff   `json:"security_diff,omitempty"`
	Diff         *models.CombinedFileDiff `json:"diff,omitempty"`
}

func (o *Orchestrator) GetEndpointDetails(ctx context.Context, projectIdentifier, websiteSlug, canonicalURL, baseVersionID, headVersionID string) (*EndpointDetails, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}

	var snap, pSnap *models.Snapshot

	// If version IDs are provided, use them for comparison
	if baseVersionID != "" && headVersionID != "" {
		o.logger.Info("Fetching endpoint details with version comparison",
			logging.Field{Key: "url", Value: canonicalURL},
			logging.Field{Key: "base_version_id", Value: baseVersionID},
			logging.Field{Key: "head_version_id", Value: headVersionID})

		// Get head snapshot
		snap, err = comps.Tracker.GetSnapshotByURLAndVersionID(ctx, canonicalURL, headVersionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get head snapshot for version %s: %w", headVersionID, err)
		}
		if snap == nil {
			return nil, fmt.Errorf("no snapshot found for URL %s at version %s", canonicalURL, headVersionID)
		}

		// Get base snapshot
		pSnap, err = comps.Tracker.GetSnapshotByURLAndVersionID(ctx, canonicalURL, baseVersionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get base snapshot for version %s: %w", baseVersionID, err)
		}
		if pSnap == nil {
			return nil, fmt.Errorf("no snapshot found for URL %s at version %s", canonicalURL, baseVersionID)
		}
	} else if baseVersionID != "" || headVersionID != "" {
		return nil, fmt.Errorf("both base_version_id and head_version_id must be provided together")
	} else {
		// Default behavior: get latest snapshot
		snap, err = comps.Tracker.GetSnapshotByURL(ctx, canonicalURL)
		if err != nil {
			return nil, err
		}
	}

	scoreResult, err := comps.Tracker.GetScoreResultFromSnapshotID(ctx, snap.ID)
	if err != nil {
		return nil, err
	}

	var (
		diff    *models.CombinedFileDiff
		secDiff *assessor.SecurityDiff
	)

	// If we already have both snapshots from version IDs, use them
	if pSnap != nil {
		if d, err := comps.Tracker.DiffSnapshots(ctx, pSnap.ID, snap.ID); err != nil {
			o.logger.Warn("Failed to compute diff between snapshots, skipping diff",
				logging.Field{Key: "base_snapshot_id", Value: pSnap.ID},
				logging.Field{Key: "head_snapshot_id", Value: snap.ID},
				logging.Field{Key: "error", Value: err.Error()})
		} else {
			diff = d
		}

		if sd, err := comps.Tracker.GetSecurityDiff(ctx, pSnap.ID, snap.ID); err != nil {
			o.logger.Warn("Failed to compute security diff between snapshots, skipping security diff",
				logging.Field{Key: "base_snapshot_id", Value: pSnap.ID},
				logging.Field{Key: "head_snapshot_id", Value: snap.ID},
				logging.Field{Key: "error", Value: err.Error()})
		} else {
			secDiff = sd
		}
	} else {
		// Default behavior: get parent version and compute diff
		parentVersionID, err := comps.Tracker.GetParentVersionID(ctx, snap.VersionID)
		if err != nil {
			o.logger.Warn("Failed to get parent version ID for version, skipping diffs",
				logging.Field{Key: "version_id", Value: snap.VersionID},
				logging.Field{Key: "error", Value: err.Error()})
		} else if parentVersionID != "" {
			pSnap, err := comps.Tracker.GetSnapshotByURLAndVersionID(ctx, canonicalURL, parentVersionID)
			if err != nil {
				o.logger.Warn("Failed to get parent snapshot for URL, skipping diffs",
					logging.Field{Key: "url", Value: canonicalURL},
					logging.Field{Key: "parent_version_id", Value: parentVersionID},
					logging.Field{Key: "error", Value: err.Error()})
			} else if pSnap != nil {
				if d, err := comps.Tracker.DiffSnapshots(ctx, pSnap.ID, snap.ID); err != nil {
					o.logger.Warn("Failed to compute diff between snapshots, skipping diff",
						logging.Field{Key: "base_snapshot_id", Value: pSnap.ID},
						logging.Field{Key: "head_snapshot_id", Value: snap.ID},
						logging.Field{Key: "error", Value: err.Error()})
				} else {
					diff = d
				}

				if sd, err := comps.Tracker.GetSecurityDiff(ctx, pSnap.ID, snap.ID); err != nil {
					o.logger.Warn("Failed to compute security diff between snapshots, skipping security diff",
						logging.Field{Key: "base_snapshot_id", Value: pSnap.ID},
						logging.Field{Key: "head_snapshot_id", Value: snap.ID},
						logging.Field{Key: "error", Value: err.Error()})
				} else {
					secDiff = sd
				}
			}
		}
	}

	return &EndpointDetails{
		Snapshot:     snap,
		ScoreResult:  scoreResult,
		SecurityDiff: secDiff,
		Diff:         diff,
	}, nil
}

func (o *Orchestrator) Close() {
	o.closedMu.Lock()
	if o.closed {
		o.closedMu.Unlock()
		return
	}
	o.closed = true
	o.closedMu.Unlock()

	o.jobsMu.Lock()
	for id, cancel := range o.jobCancels {
		if cancel != nil {
			cancel()
		}
		delete(o.jobCancels, id)
	}
	o.cleanupFinishedJobsLocked()
	o.jobsMu.Unlock()

	o.siteCompMutex.Lock()
	for id, sc := range o.siteComponentsCache {
		if sc != nil {
			if err := sc.Close(); err != nil && o.logger != nil {
				o.logger.Warn("failed to close site components",
					logging.Field{Key: "website_id", Value: id},
					logging.Field{Key: "error", Value: err.Error()},
				)
			}
		}
		delete(o.siteComponentsCache, id)
	}
	o.siteCompMutex.Unlock()
}
