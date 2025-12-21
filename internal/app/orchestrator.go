package app

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/tracker/models"
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

	jobsMu     sync.Mutex
	jobs       map[string]*Job
	jobCancels map[string]context.CancelFunc
}

// NewOrchestrator ties together config, registry and logger.
func NewOrchestrator(cfg *Config, reg *registry.Registry, logger logging.Logger) *Orchestrator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Orchestrator{
		cfg:      cfg,
		registry: reg,
		logger:   logger,
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

func (o *Orchestrator) StartFetchJob(ctx context.Context, project, site, status string, limit int) (*Job, error) {
	o.ensureJobMaps()

	jobID := uuid.New().String()
	now := time.Now().UTC()

	job := &Job{
		ID:        jobID,
		Type:      "fetch",
		Project:   project,
		Website:   site,
		Status:    JobPending,
		StartedAt: now,
		Events:    make(chan JobEvent, 16),
	}

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
		defer func() {
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.EndedAt = time.Now().UTC()
			}
			o.jobsMu.Unlock()
			o.deleteCancel(jobID)

			// Close events channel so websocket loop can terminate cleanly
			o.jobsMu.Lock()
			j := o.jobs[jobID]
			o.jobsMu.Unlock()
			if j != nil && j.Events != nil {
				close(j.Events)
			}
		}()

		// Mark running
		o.jobsMu.Lock()
		if j, ok := o.jobs[jobID]; ok {
			j.Status = JobRunning
		}
		o.jobsMu.Unlock()
		o.emitJobEvent(jobID, JobEvent{
			JobID:  jobID,
			Type:   JobEventStatus,
			Status: JobRunning,
		})

		overview, err := o.FetchWebsiteEndpoints(jobCtx, project, site, status, limit)
		if err != nil {
			select {
			case <-jobCtx.Done():
				o.jobsMu.Lock()
				if j, ok := o.jobs[jobID]; ok {
					j.Status = JobCanceled
					j.Error = jobCtx.Err().Error()
				}
				o.jobsMu.Unlock()
				o.emitJobEvent(jobID, JobEvent{
					JobID:  jobID,
					Type:   JobEventStatus,
					Status: JobCanceled,
					Error:  jobCtx.Err().Error(),
				})
			default:
				o.jobsMu.Lock()
				if j, ok := o.jobs[jobID]; ok {
					j.Status = JobFailed
					j.Error = err.Error()
				}
				o.jobsMu.Unlock()
				o.emitJobEvent(jobID, JobEvent{
					JobID:  jobID,
					Type:   JobEventStatus,
					Status: JobFailed,
					Error:  err.Error(),
				})
			}
			return
		}

		select {
		case <-jobCtx.Done():
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.Status = JobCanceled
				j.Error = jobCtx.Err().Error()
			}
			o.jobsMu.Unlock()
			o.emitJobEvent(jobID, JobEvent{
				JobID:  jobID,
				Type:   JobEventStatus,
				Status: JobCanceled,
				Error:  jobCtx.Err().Error(),
			})
		default:
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.Status = JobDone
				j.SecurityOverview = overview
			}
			o.jobsMu.Unlock()
			o.emitJobEvent(jobID, JobEvent{
				JobID:  jobID,
				Type:   JobEventResult,
				Status: JobDone,
			})
		}
	}()

	return job, nil
}

func (o *Orchestrator) StartEnumerateJob(ctx context.Context, project, site string, concurrency int) (*Job, error) {
	o.ensureJobMaps()

	jobID := uuid.New().String()
	now := time.Now().UTC()

	job := &Job{
		ID:        jobID,
		Type:      "enumerate",
		Project:   project,
		Website:   site,
		Status:    JobPending,
		StartedAt: now,
		Events:    make(chan JobEvent, 16),
	}

	o.setJob(job)

	jobCtx, cancel := context.WithCancel(ctx)
	o.setCancel(jobID, cancel)

	o.emitJobEvent(jobID, JobEvent{
		JobID:  jobID,
		Type:   JobEventStatus,
		Status: JobPending,
	})

	go func() {
		defer func() {
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.EndedAt = time.Now().UTC()
			}
			o.jobsMu.Unlock()
			o.deleteCancel(jobID)

			o.jobsMu.Lock()
			j := o.jobs[jobID]
			o.jobsMu.Unlock()
			if j != nil && j.Events != nil {
				close(j.Events)
			}
		}()

		o.jobsMu.Lock()
		if j, ok := o.jobs[jobID]; ok {
			j.Status = JobRunning
		}
		o.jobsMu.Unlock()
		o.emitJobEvent(jobID, JobEvent{
			JobID:  jobID,
			Type:   JobEventStatus,
			Status: JobRunning,
		})

		urls, err := o.EnumerateWebsite(jobCtx, project, site, concurrency)
		if err != nil {
			select {
			case <-jobCtx.Done():
				o.jobsMu.Lock()
				if j, ok := o.jobs[jobID]; ok {
					j.Status = JobCanceled
					j.Error = jobCtx.Err().Error()
				}
				o.jobsMu.Unlock()
				o.emitJobEvent(jobID, JobEvent{
					JobID:  jobID,
					Type:   JobEventStatus,
					Status: JobCanceled,
					Error:  jobCtx.Err().Error(),
				})
			default:
				o.jobsMu.Lock()
				if j, ok := o.jobs[jobID]; ok {
					j.Status = JobFailed
					j.Error = err.Error()
				}
				o.jobsMu.Unlock()
				o.emitJobEvent(jobID, JobEvent{
					JobID:  jobID,
					Type:   JobEventStatus,
					Status: JobFailed,
					Error:  err.Error(),
				})
			}
			return
		}

		select {
		case <-jobCtx.Done():
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.Status = JobCanceled
				j.Error = jobCtx.Err().Error()
			}
			o.jobsMu.Unlock()
			o.emitJobEvent(jobID, JobEvent{
				JobID:  jobID,
				Type:   JobEventStatus,
				Status: JobCanceled,
				Error:  jobCtx.Err().Error(),
			})
		default:
			o.jobsMu.Lock()
			if j, ok := o.jobs[jobID]; ok {
				j.Status = JobDone
				j.EnumeratedURLs = urls
			}
			o.jobsMu.Unlock()
			o.emitJobEvent(jobID, JobEvent{
				JobID:  jobID,
				Type:   JobEventResult,
				Status: JobDone,
			})
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

func (o *Orchestrator) EnumerateWebsite(ctx context.Context, projectIdentifier, websiteSlug string, concurrency int) ([]string, error) {
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
	spider := enumerator.NewSpider(concurrency, comps.WebClient, o.logger)
	targets, err := spider.Enumerate(ctx, web.Origin)
	if err != nil {
		return nil, err
	}

	_, err = comps.Index.AddEndpoints(ctx, targets, "enumerator")
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
	return comps.Index.ListEndpoints(ctx, status, limit)
}

func (o *Orchestrator) FetchWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int) (*assessor.SecurityDiffOverview, error) {
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
		if err := comps.Fetcher.FetchFromIndex(ctx, status, limit); err != nil {
			return nil, err
		}
		return nil, nil
	}

	prevHeadID, err := comps.Tracker.ReadHEAD()
	if err != nil {
		return nil, err
	}

	if err := comps.Fetcher.FetchFromIndex(ctx, status, limit); err != nil {
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
	SecurityDiff *assessor.SecurityDiff   `json:"diff_with_prev,omitempty"`
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
