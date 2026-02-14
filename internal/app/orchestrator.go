package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
)

type JobEventType string

const (
	JobEventStatus   JobEventType = "status"
	JobEventProgress JobEventType = "progress"
	JobEventResult   JobEventType = "result"
)

type JobEvent struct {
	JobID string       `json:"job_id"`
	Type  JobEventType `json:"type"`

	// For status changes
	Status JobStatus `json:"status,omitempty"`
	Error  string    `json:"error,omitempty"`

	// For progress (optional fields)
	Processed int `json:"processed,omitempty"`
	Total     int `json:"total,omitempty"`
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
	ID        string        `json:"id"`
	Type      string        `json:"type"` // "fetch" | "enumerate"
	Project   string        `json:"project"`
	Website   string        `json:"website"`
	Status    JobStatus     `json:"status"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Events    chan JobEvent `json:"-"`

	// Optional results:
	SecurityOverview *assessor.SecurityDiffOverview `json:"security_overview,omitempty"`
	EnumeratedURLs   []string                       `json:"enumerated_urls,omitempty"`
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

// NewOrchestrator ties together config, registry and logger.
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

	// Non-blocking send; drop if buffer is full.
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
	// caller MUST hold o.jobsMu
	if o.jobRetentionTime <= 0 {
		return
	}

	now := time.Now().UTC()
	for id, job := range o.jobs {
		// Only remove jobs that are finished and past retention
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

	// Cleanup old jobs while we hold the lock
	o.cleanupFinishedJobsLocked()

	// Capture events chan before unlocking
	var events chan JobEvent
	if j, ok := o.jobs[jobID]; ok && j != nil {
		events = j.Events
	}
	o.jobsMu.Unlock()

	o.deleteCancel(jobID)

	if events != nil {
		close(events)
	}
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

func (o *Orchestrator) StartFetchJob(ctx context.Context, project, site, status string, limit int) (*Job, error) {
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

	go func() {
		defer o.finishJob(jobID)
		// Mark running
		o.setJobStatus(jobID, JobRunning, nil)

		cb := o.progressCallback(jobID)
		o.logger.Info("Starting fetch job", logging.Field{Key: "job_id", Value: jobID}, logging.Field{Key: "website", Value: site}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
		overview, err := o.FetchWebsiteEndpoints(jobCtx, project, site, status, limit, cb)
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

	return job, nil
}

func (o *Orchestrator) StartEnumerateJob(ctx context.Context, project, site string, maxDepth int) (*Job, error) {
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

	go func() {
		defer o.finishJob(jobID)
		o.setJobStatus(jobID, JobRunning, nil)

		cb := o.progressCallback(jobID)
		urls, err := o.EnumerateWebsite(jobCtx, project, site, maxDepth, cb)
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

	return job, nil
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
	return j
}

// ListJobs returns a list of current jobs.
func (o *Orchestrator) ListJobs() []*Job {
	o.jobsMu.Lock()
	defer o.jobsMu.Unlock()

	jobs := make([]*Job, 0, len(o.jobs))
	for _, j := range o.jobs {
		if j != nil {
			jobs = append(jobs, j)
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

func (o *Orchestrator) EnumerateWebsite(ctx context.Context, projectIdentifier, websiteSlug string, maxDepth int, cb utils.ProgressCallback) ([]string, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}

	// Currently, we only have a spider enumerator.
	// When multiple enumerators are added, we can make this configurable.
	spider := enumerator.NewSpider(maxDepth, comps.WebClient, o.logger)
	targets, err := spider.Enumerate(ctx, web.Origin, cb)
	if err != nil {
		return nil, err
	}

	_, err = comps.Index.AddEndpoints(ctx, targets, "spider-enumerator")
	if err != nil {
		return nil, err
	}
	return targets, nil
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

func (o *Orchestrator) FetchWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int, cb utils.ProgressCallback) (*assessor.SecurityDiffOverview, error) {
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

	headExists, err := comps.Tracker.HEADExists()
	if err != nil {
		return nil, err
	}

	if !headExists {
		// First fetch, no previous version to compare against
		if err := comps.Fetcher.FetchFromIndex(ctx, status, limit, cb); err != nil {
			return nil, err
		}
		return nil, nil
	}

	prevHeadID, err := comps.Tracker.ReadHEAD()
	if err != nil {
		return nil, err
	}

	o.logger.Info("Starting fetch of website endpoints", logging.Field{Key: "website_id", Value: web.ID}, logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit}, logging.Field{Key: "previous_head_id", Value: prevHeadID})
	if err := comps.Fetcher.FetchFromIndex(ctx, status, limit, cb); err != nil {
		return nil, err
	}

	newHeadID, err := comps.Tracker.ReadHEAD()
	if err != nil {
		return nil, err
	}

	if prevHeadID == "" || prevHeadID == newHeadID {
		return nil, nil // no previous or no change
	}

	return comps.Tracker.GetSecurityDiffOverview(ctx, prevHeadID, newHeadID)
}

type EndpointDetails struct {
	Snapshot     *models.Snapshot         `json:"snapshot"`
	ScoreResult  *assessor.ScoreResult    `json:"score_result,omitempty"`
	SecurityDiff *assessor.SecurityDiff   `json:"security_diff,omitempty"`
	Diff         *models.CombinedFileDiff `json:"diff,omitempty"`
}

func (o *Orchestrator) GetEndpointDetails(ctx context.Context, projectIdentifier, websiteSlug, canonicalURL string) (*EndpointDetails, error) {
	web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
	if err != nil {
		return nil, err
	}
	comps, err := o.siteComponentsFor(ctx, web)
	if err != nil {
		return nil, err
	}

	snap, err := comps.Tracker.GetSnapshotByURL(ctx, canonicalURL)
	if err != nil {
		return nil, err
	}

	scoreResult, err := comps.Tracker.GetScoreResultFromSnapshotID(ctx, snap.ID)
	if err != nil {
		return nil, err
	}

	var (
		diff    *models.CombinedFileDiff
		secDiff *assessor.SecurityDiff
	)

	pVerID, err := comps.Tracker.GetParentVersionID(ctx, snap.VersionID)
	if err != nil {
		o.logger.Warn("Failed to get parent version ID for version, skipping diffs",
			logging.Field{Key: "version_id", Value: snap.VersionID},
			logging.Field{Key: "error", Value: err.Error()})
	} else if pVerID != "" {
		pSnap, err := comps.Tracker.GetSnapshotByURLAndVersionID(ctx, canonicalURL, pVerID)
		if err != nil {
			o.logger.Warn("Failed to get parent snapshot for URL, skipping diffs",
				logging.Field{Key: "url", Value: canonicalURL},
				logging.Field{Key: "parent_version_id", Value: pVerID},
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

	return &EndpointDetails{
		Snapshot:     snap,
		ScoreResult:  scoreResult,
		SecurityDiff: secDiff,
		Diff:         diff,
	}, nil
}

func (o *Orchestrator) Close() {
	// Mark closed so no new jobs should be started.
	o.closedMu.Lock()
	if o.closed {
		o.closedMu.Unlock()
		return
	}
	o.closed = true
	o.closedMu.Unlock()

	// Cancel all running jobs.
	o.jobsMu.Lock()
	for id, cancel := range o.jobCancels {
		if cancel != nil {
			cancel()
		}
		delete(o.jobCancels, id)
	}
	o.cleanupFinishedJobsLocked()
	o.jobsMu.Unlock()

	// Close all site components (DB connections, HTTP clients, etc.).
	o.siteCompMutex.Lock()
	for id, sc := range o.siteComponentsCache {
		if sc != nil {
			// Ignore individual errors on close; log if you want.
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
