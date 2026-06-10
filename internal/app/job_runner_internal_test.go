package app

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// pollUntilTerminal waits for the job to reach a terminal status, returning
// the terminal job or failing the test on timeout.
func pollUntilTerminal(t *testing.T, o *Orchestrator, jobID string, timeout time.Duration) *Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		j := o.GetJob(jobID)
		if j != nil && (j.Status == JobDone || j.Status == JobFailed || j.Status == JobCanceled) {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach a terminal status within %s", jobID, timeout)
	return nil
}

// TestRunJob_RecoversPanicAndMarksJobFailed proves that a work func that
// panics does not crash the process: runJob's recover defer promotes the job
// to JobFailed with a "panic" error message.
func TestRunJob_RecoversPanicAndMarksJobFailed(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	job := o.jobs.newJob("fetch", "proj", "site")
	work := func(_ context.Context) (func(), error) {
		panic("boom in job body")
	}

	o.runJob(context.Background(), job, work)

	final := pollUntilTerminal(t, o, job.ID, 2*time.Second)
	if final.Status != JobFailed {
		t.Fatalf("Job.Status = %q, want %q", final.Status, JobFailed)
	}
	if !strings.Contains(final.Error, "panic") {
		t.Errorf("Job.Error = %q; expected it to mention %q", final.Error, "panic")
	}
}

// TestRunJob_CancelMapsToCanceledStatus proves that when the work func returns
// the context error after cancellation, runJob records JobCanceled (not
// JobFailed).
func TestRunJob_CancelMapsToCanceledStatus(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	job := o.jobs.newJob("fetch", "proj", "site")
	work := func(ctx context.Context) (func(), error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	o.runJob(context.Background(), job, work)

	o.CancelJob(job.ID)

	final := pollUntilTerminal(t, o, job.ID, 2*time.Second)
	if final.Status != JobCanceled {
		t.Fatalf("Job.Status = %q, want %q", final.Status, JobCanceled)
	}
}

// TestClose_WaitsForRunningJobGoroutine proves Close blocks until in-flight
// job goroutines return: the work func signals "started", sleeps, then sets a
// shared flag before returning. Close must observe the flag as set.
func TestClose_WaitsForRunningJobGoroutine(t *testing.T) {
	t.Parallel()
	o := newTestOrchestrator(t)

	started := make(chan struct{})
	var finished atomic.Bool
	work := func(_ context.Context) (func(), error) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		finished.Store(true)
		return func() {}, nil
	}

	job := o.jobs.newJob("fetch", "proj", "site")
	o.runJob(context.Background(), job, work)

	<-started
	o.Close()

	if !finished.Load() {
		t.Fatal("Close returned before the running job goroutine finished")
	}
}
