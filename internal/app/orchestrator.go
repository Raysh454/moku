package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

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

// Orchestrator is the facade composing the platform's pillars. It delegates
// per-site component caching to siteComponentsCatalog, job bookkeeping to
// jobManager, and event fan-out to subscriberBroker, while owning the
// composition between them (job state transitions emit enriched events).
type Orchestrator struct {
	cfg      *Config
	registry *registry.Registry
	logger   logging.Logger

	sites  *siteComponentsCatalog
	jobs   *jobManager
	broker *subscriberBroker

	closedMu sync.Mutex
	closed   bool
}

func NewOrchestrator(cfg *Config, reg *registry.Registry, logger logging.Logger) *Orchestrator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	buildSite := func(ctx context.Context, web registry.Website) (*SiteComponents, error) {
		return NewSiteComponents(ctx, cfg, web, logger)
	}
	return &Orchestrator{
		cfg:      cfg,
		registry: reg,
		logger:   logger,
		sites:    newSiteComponentsCatalog(buildSite, logger),
		jobs:     newJobManager(cfg.JobRetentionTime),
		broker:   newSubscriberBroker(logger),
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

// isClosed reports whether Close has begun; a closed orchestrator rejects
// new jobs.
func (o *Orchestrator) isClosed() bool {
	o.closedMu.Lock()
	defer o.closedMu.Unlock()
	return o.closed
}

func (o *Orchestrator) Subscribe(ctx context.Context) chan JobEvent {
	return o.broker.subscribe(ctx)
}

func (o *Orchestrator) Unsubscribe(ch chan JobEvent) {
	o.broker.unsubscribe(ch)
}

func (o *Orchestrator) emitJobEvent(jobID string, ev JobEvent) {
	job := o.jobs.get(jobID)
	if job == nil {
		if o.logger != nil {
			o.logger.Debug("orchestrator: job not found for event emission", logging.Field{Key: "job_id", Value: jobID})
		}
		return
	}

	// Enrich event with project/website context
	ev.Project = job.Project
	ev.Website = job.Website
	ev.JobID = jobID

	o.broker.publish(ev)
}

func (o *Orchestrator) setJobStatus(jobID string, status JobStatus, err error) {
	o.jobs.setStatus(jobID, status, err)

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
	o.jobs.setResult(jobID, overview, urls)

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
	o.jobs.setScanResult(jobID, scan)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventResult,
		Status: JobDone,
	})
}

func (o *Orchestrator) progressCallback(jobID string) utils.ProgressCallback {
	return func(processed, failed, total int) {
		o.emitJobEvent(jobID, JobEvent{
			JobID:     jobID,
			Type:      JobEventProgress,
			Processed: processed,
			Failed:    failed,
			Total:     total,
		})
	}
}

func (o *Orchestrator) StartFetchJob(ctx context.Context, project, site, status string, limit int, cfg *api.FetchConfig) (*Job, error) {
	o.logger.Info("orchestrator: Starting fetch job", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
	if o.isClosed() {
		return nil, fmt.Errorf("orchestrator is closed")
	}

	job := o.jobs.newJob("fetch", project, site)
	jobID := job.ID

	o.jobs.set(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.jobs.registerCancel(jobID, cancel)

	// Emit initial pending event
	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.jobs.finish(jobID)
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
	if o.isClosed() {
		return nil, fmt.Errorf("orchestrator is closed")
	}

	job := o.jobs.newJob("enumerate", project, site)
	jobID := job.ID

	o.jobs.set(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.jobs.registerCancel(jobID, cancel)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.jobs.finish(jobID)
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

	if o.isClosed() {
		return nil, fmt.Errorf("orchestrator is closed")
	}

	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.sites.componentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	if comps.Analyzer == nil {
		return nil, fmt.Errorf("StartScanJob: site components have no analyzer")
	}

	job := o.jobs.newJob("scan", projectIdentifier, websiteSlug)
	jobID := job.ID
	o.jobs.set(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.jobs.registerCancel(jobID, cancel)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})
	jobSnapshot := cloneJob(job)

	go func() {
		defer o.jobs.finish(jobID)
		o.setJobStatus(jobID, JobRunning, nil)

		result, err := comps.Analyzer.ScanAndWait(jobCtx, req, o.cfg.AnalyzerCfg.DefaultPoll)
		if err != nil {
			select {
			case <-jobCtx.Done():
				o.setJobStatus(jobID, JobCanceled, jobCtx.Err())
			default:
				// Transport-level failure talking to the analyzer sidecar gets a
				// dedicated, user-facing failure message so operators can tell
				// "scanner crashed" from "scanner not running". Every other error
				// (in-band sidecar status, decode failure, scan-engine error) falls
				// through to the generic JobFailed path.
				if errors.Is(err, analyzer.ErrSidecarUnreachable) {
					o.setJobStatus(jobID, JobFailed, fmt.Errorf("vulnerability analyzer sidecar offline: %w", err))
				} else {
					o.setJobStatus(jobID, JobFailed, err)
				}
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
	comps, err := o.sites.componentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	if comps.Analyzer == nil {
		return nil, fmt.Errorf("no analyzer configured for site %s/%s", projectIdentifier, websiteSlug)
	}
	return comps.Analyzer, nil
}

func (o *Orchestrator) CancelJob(jobID string) {
	o.jobs.cancel(jobID)
}

func (o *Orchestrator) GetJob(jobID string) *Job {
	return o.jobs.get(jobID)
}

func (o *Orchestrator) ListJobs() []*Job {
	return o.jobs.list()
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

func (o *Orchestrator) DeleteProject(ctx context.Context, identifier string) error {
	return o.registry.DeleteProject(ctx, identifier)
}

func (o *Orchestrator) DeleteWebsite(ctx context.Context, projectIdentifier, websiteSlug string) error {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return err
	}

	// Job.Project / Job.Website hold the URL-path identifier (slug) passed
	// to the Start*Job methods, not the website's UUID. Filter on the same
	// slug here, otherwise the cancel/wait loops match no jobs and we race
	// the running goroutines into the eviction below.
	o.jobs.cancelWebsiteJobs(projectIdentifier, web.Slug)
	if err := o.jobs.waitForWebsiteJobs(ctx, projectIdentifier, web.Slug); err != nil {
		return fmt.Errorf("wait for website jobs: %w", err)
	}

	comps := o.sites.evict(web.ID)
	if comps != nil {
		if err := comps.Close(); err != nil {
			return fmt.Errorf("close site components: %w", err)
		}
	}

	return o.registry.DeleteWebsite(ctx, projectIdentifier, websiteSlug)
}

// GetWebsiteIndexer returns the indexer for a website.
func (o *Orchestrator) GetWebsiteIndexer(ctx context.Context, projectIdentifier, websiteSlug string) (*indexer.Index, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.sites.componentsFor(ctx, web)
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
	comps, err := o.sites.componentsFor(ctx, web)
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
	comps, err := o.sites.componentsFor(ctx, web)
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
		spider := enumerator.NewSpider(maxDepth, wc, o.logger)
		if cfg.Spider.MaxPages > 0 {
			spider.MaxPages = cfg.Spider.MaxPages
		}
		enumerators = append(enumerators, spider)
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

	// Default to spider if nothing specified. The constructor applies the
	// default page cap; there is no per-request override in this branch.
	if len(enumerators) == 0 {
		spider := enumerator.NewSpider(4, wc, o.logger)
		enumerators = append(enumerators, spider)
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
	comps, err := o.sites.componentsFor(ctx, web)
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
	comps, err := o.sites.componentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	o.logger.Info("Listing versions", logging.Field{Key: "website_id", Value: web.ID}, logging.Field{Key: "limit", Value: limit})
	return comps.Tracker.ListVersions(ctx, limit)
}

func (o *Orchestrator) GetWebsiteSecurityDiffOverview(
	ctx context.Context,
	projectIdentifier,
	websiteSlug,
	baseVersionID,
	headVersionID string,
) (*assessor.SecurityDiffOverview, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.sites.componentsFor(ctx, web)
	if err != nil {
		return nil, err
	}
	if headVersionID == "" {
		return nil, fmt.Errorf("head version is required")
	}
	o.logger.Info(
		"Getting security overview",
		logging.Field{Key: "website_id", Value: web.ID},
		logging.Field{Key: "base_version_id", Value: baseVersionID},
		logging.Field{Key: "head_version_id", Value: headVersionID},
	)
	return comps.Tracker.GetSecurityDiffOverview(ctx, baseVersionID, headVersionID)
}

func (o *Orchestrator) FetchWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int, cfg *api.FetchConfig, cb utils.ProgressCallback) (*assessor.SecurityDiffOverview, error) {
	if status == "" {
		status = "*"
	}

	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}

	comps, err := o.sites.componentsFor(ctx, web)
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
	comps, err := o.sites.componentsFor(ctx, web)
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

	o.jobs.shutdown()
	o.sites.closeAll()
	o.broker.close()
}
