package app

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/assessor"
)

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

// websiteJobsPollInterval is how often waitForWebsiteJobs re-checks whether a
// website still has active jobs.
const websiteJobsPollInterval = 50 * time.Millisecond

// jobManager owns the in-memory job table: job records, their cancel
// functions, and retention-based pruning of finished jobs.
type jobManager struct {
	mu        sync.Mutex
	jobs      map[string]*Job
	cancels   map[string]context.CancelFunc
	retention time.Duration
}

func newJobManager(retention time.Duration) *jobManager {
	return &jobManager{
		jobs:      make(map[string]*Job),
		cancels:   make(map[string]context.CancelFunc),
		retention: retention,
	}
}

// newJob builds a pending Job record for the given type and target.
func (m *jobManager) newJob(jobType, project, site string) *Job {
	return &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Project:   project,
		Website:   site,
		Status:    JobPending,
		StartedAt: time.Now().UTC(),
	}
}

// set stores (or replaces) a job record.
func (m *jobManager) set(job *Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
}

// get returns a copy of the job, or nil when the ID is unknown.
func (m *jobManager) get(jobID string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	return cloneJob(j)
}

// list returns copies of every stored job.
func (m *jobManager) list() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j != nil {
			jobs = append(jobs, cloneJob(j))
		}
	}
	return jobs
}

// setStatus records a status transition (and optional error) on the job.
func (m *jobManager) setStatus(jobID string, status JobStatus, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = status
		if err != nil {
			j.Error = err.Error()
		}
	}
}

// setResult marks the job done and attaches the fetch/enumerate payload.
func (m *jobManager) setResult(jobID string, overview *assessor.SecurityDiffOverview, urls []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = JobDone
		j.SecurityOverview = overview
		j.EnumeratedURLs = urls
	}
}

// setScanResult marks the job done and attaches the analyzer payload.
// Separate from setResult because scan jobs carry a different payload type
// than fetch/enumerate jobs.
func (m *jobManager) setScanResult(jobID string, scan *analyzer.ScanResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[jobID]; ok {
		j.Status = JobDone
		j.ScanResult = scan
	}
}

// registerCancel associates a cancel func with the job so cancel can reach
// the goroutine driving it.
func (m *jobManager) registerCancel(jobID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancels[jobID] = cancel
}

// cancel invokes the job's registered cancel func, if any. The func is
// invoked outside the lock.
func (m *jobManager) cancel(jobID string) {
	m.mu.Lock()
	cancel := m.cancels[jobID]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// finish stamps the job's end time, prunes expired finished jobs, and
// releases the job's cancel func.
func (m *jobManager) finish(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[jobID]; ok {
		j.EndedAt = time.Now().UTC()
	}
	m.cleanupFinishedLocked()
	delete(m.cancels, jobID)
}

// cleanupFinishedLocked removes finished jobs whose end time exceeds the
// retention window. Active jobs are never removed, and a non-positive
// retention disables pruning entirely. Callers must hold mu.
func (m *jobManager) cleanupFinishedLocked() {
	if m.retention <= 0 {
		return
	}

	now := time.Now().UTC()
	for id, job := range m.jobs {
		if job == nil {
			delete(m.jobs, id)
			continue
		}

		if job.Status != JobDone && job.Status != JobFailed && job.Status != JobCanceled {
			continue
		}
		if job.EndedAt.IsZero() {
			continue
		}

		if now.Sub(job.EndedAt) > m.retention {
			delete(m.jobs, id)
		}
	}
}

// cancelWebsiteJobs invokes the cancel funcs of every job targeting the
// given project/website pair.
func (m *jobManager) cancelWebsiteJobs(projectID, websiteSlug string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for jobID, job := range m.jobs {
		if job == nil || job.Project != projectID || job.Website != websiteSlug {
			continue
		}
		if cancel := m.cancels[jobID]; cancel != nil {
			cancel()
		}
	}
}

// waitForWebsiteJobs blocks until no job targeting the project/website pair
// is still active, or until ctx is done.
func (m *jobManager) waitForWebsiteJobs(ctx context.Context, projectID, websiteSlug string) error {
	ticker := time.NewTicker(websiteJobsPollInterval)
	defer ticker.Stop()

	for {
		if !m.hasActiveWebsiteJobs(projectID, websiteSlug) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *jobManager) hasActiveWebsiteJobs(projectID, websiteSlug string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.jobs {
		if job == nil || job.Project != projectID || job.Website != websiteSlug {
			continue
		}
		if job.Status != JobDone && job.Status != JobFailed && job.Status != JobCanceled {
			return true
		}
	}
	return false
}

// shutdown cancels every registered cancel func and prunes expired finished
// jobs. Job records still inside the retention window remain readable.
func (m *jobManager) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cancel := range m.cancels {
		if cancel != nil {
			cancel()
		}
		delete(m.cancels, id)
	}
	m.cleanupFinishedLocked()
}
