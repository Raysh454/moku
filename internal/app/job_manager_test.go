package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

const testRetention = time.Minute

// expiredFinishedJob fabricates a job that ended longer ago than testRetention.
func expiredFinishedJob(id string, status JobStatus) *Job {
	return &Job{ID: id, Status: status, EndedAt: time.Now().UTC().Add(-2 * testRetention)}
}

// finishAnyJob stores a throwaway job and finishes it, which is the trigger
// for retention-based pruning.
func finishAnyJob(m *jobManager) {
	trigger := m.newJob("fetch", "p", "s")
	m.set(trigger)
	m.finish(trigger.ID)
}

func TestJobManager_GetReturnsNilForUnknownJob(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)

	if got := m.get("nonexistent"); got != nil {
		t.Errorf("expected nil for unknown job, got %+v", got)
	}
}

func TestJobManager_NewJobStartsPendingWithIdentity(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)

	job := m.newJob("fetch", "proj", "site")

	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.Type != "fetch" || job.Project != "proj" || job.Website != "site" {
		t.Errorf("unexpected job identity: %+v", job)
	}
	if job.Status != JobPending {
		t.Errorf("status = %q, want %q", job.Status, JobPending)
	}
	if job.StartedAt.IsZero() {
		t.Error("expected StartedAt to be stamped")
	}
}

func TestJobManager_GetReturnsACopySoCallersCannotMutateStoredState(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "proj", "site")
	m.set(job)

	m.get(job.ID).Status = JobFailed

	if got := m.get(job.ID).Status; got != JobPending {
		t.Errorf("stored status = %q, want %q (mutating a returned job must not affect the store)", got, JobPending)
	}
}

func TestJobManager_ListReturnsEveryStoredJob(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	first := m.newJob("fetch", "proj", "site")
	second := m.newJob("enumerate", "proj", "site")
	m.set(first)
	m.set(second)

	jobs := m.list()

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	seen := map[string]bool{}
	for _, j := range jobs {
		seen[j.ID] = true
	}
	if !seen[first.ID] || !seen[second.ID] {
		t.Errorf("listed IDs %v missing one of %q, %q", seen, first.ID, second.ID)
	}
}

func TestJobManager_SetStatusRecordsStatusAndError(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "proj", "site")
	m.set(job)

	m.setStatus(job.ID, JobFailed, errors.New("boom"))

	got := m.get(job.ID)
	if got.Status != JobFailed {
		t.Errorf("status = %q, want %q", got.Status, JobFailed)
	}
	if got.Error != "boom" {
		t.Errorf("error = %q, want %q", got.Error, "boom")
	}
}

func TestJobManager_CancelInvokesRegisteredCancelFunc(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "proj", "site")
	m.set(job)
	invoked := false
	m.registerCancel(job.ID, func() { invoked = true })

	m.cancel(job.ID)

	if !invoked {
		t.Error("expected the registered cancel func to be invoked")
	}
}

func TestJobManager_CancelIsANoOpForUnknownJob(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)

	// Must not panic.
	m.cancel("does-not-exist")
}

func TestJobManager_FinishStampsEndTimeAndReleasesCancelFunc(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "proj", "site")
	m.set(job)
	invoked := false
	m.registerCancel(job.ID, func() { invoked = true })

	m.finish(job.ID)

	if m.get(job.ID).EndedAt.IsZero() {
		t.Error("expected EndedAt to be stamped on finish")
	}
	m.cancel(job.ID)
	if invoked {
		t.Error("cancel func must be released on finish and never fire afterwards")
	}
}

func TestJobManager_FinishPrunesExpiredFinishedJobsButNeverActiveOnes(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	m.set(expiredFinishedJob("expired-done", JobDone))
	// An active job is kept even with an ancient end time: pruning is
	// gated on terminal status first.
	m.set(&Job{ID: "still-running", Status: JobRunning, EndedAt: time.Now().UTC().Add(-2 * testRetention)})

	finishAnyJob(m)

	if m.get("expired-done") != nil {
		t.Error("expected the expired finished job to be pruned")
	}
	if m.get("still-running") == nil {
		t.Error("an active job must never be pruned")
	}
}

func TestJobManager_FinishKeepsFinishedJobsInsideRetentionWindow(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	m.set(&Job{ID: "recent-done", Status: JobDone, EndedAt: time.Now().UTC()})

	finishAnyJob(m)

	if m.get("recent-done") == nil {
		t.Error("a finished job inside the retention window must be kept")
	}
}

func TestJobManager_NonPositiveRetentionDisablesPruning(t *testing.T) {
	t.Parallel()
	m := newJobManager(0)
	m.set(expiredFinishedJob("ancient-done", JobDone))

	finishAnyJob(m)

	if m.get("ancient-done") == nil {
		t.Error("with retention <= 0 no job may ever be pruned")
	}
}

func TestJobManager_PruningSkipsFinishedJobsWithoutEndTime(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	m.set(&Job{ID: "done-no-end", Status: JobDone})

	finishAnyJob(m)

	if m.get("done-no-end") == nil {
		t.Error("a finished job without an end time must be kept")
	}
}

func TestJobManager_ShutdownCancelsEveryRegisteredJob(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	first := m.newJob("fetch", "proj", "site")
	second := m.newJob("scan", "proj", "site")
	m.set(first)
	m.set(second)
	canceled := map[string]bool{}
	m.registerCancel(first.ID, func() { canceled[first.ID] = true })
	m.registerCancel(second.ID, func() { canceled[second.ID] = true })

	m.shutdown()

	if !canceled[first.ID] || !canceled[second.ID] {
		t.Errorf("expected every registered job to be canceled, got %v", canceled)
	}
}

func TestJobManager_CancelWebsiteJobsCancelsOnlyMatchingWebsite(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	match := m.newJob("fetch", "p1", "s1")
	other := m.newJob("fetch", "p1", "s2")
	m.set(match)
	m.set(other)
	canceled := map[string]bool{}
	m.registerCancel(match.ID, func() { canceled[match.ID] = true })
	m.registerCancel(other.ID, func() { canceled[other.ID] = true })

	m.cancelWebsiteJobs("p1", "s1")

	if !canceled[match.ID] {
		t.Error("expected the matching website's job to be canceled")
	}
	if canceled[other.ID] {
		t.Error("a job for a different website must not be canceled")
	}
}

func TestJobManager_WaitForWebsiteJobsReturnsOnceJobsReachTerminalStatus(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "p1", "s1")
	m.set(job)
	m.setStatus(job.ID, JobRunning, nil)

	go func() {
		time.Sleep(2 * websiteJobsPollInterval)
		m.setStatus(job.ID, JobDone, nil)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.waitForWebsiteJobs(ctx, "p1", "s1"); err != nil {
		t.Errorf("waitForWebsiteJobs: %v (expected nil once the job finished)", err)
	}
}

func TestJobManager_WaitForWebsiteJobsHonorsContextCancellation(t *testing.T) {
	t.Parallel()
	m := newJobManager(testRetention)
	job := m.newJob("fetch", "p1", "s1")
	m.set(job)
	m.setStatus(job.ID, JobRunning, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*websiteJobsPollInterval)
	defer cancel()

	err := m.waitForWebsiteJobs(ctx, "p1", "s1")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded while a job stays active", err)
	}
}
