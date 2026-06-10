package app

import (
	"context"
	"fmt"
)

// jobWork performs the body of a job. On success it returns a commit closure
// that records the result (e.g. setJobResult / setScanJobResult); runJob
// invokes commit only when the job was not canceled. A non-nil error aborts
// the job, mapped to JobCanceled when the context is done or JobFailed
// otherwise. The commit return is ignored on error.
type jobWork func(ctx context.Context) (commit func(), err error)

// runJob drives the shared lifecycle for every job kind: it registers the job
// and its cancel func, emits the initial JobPending event, snapshots the job
// for the caller, then runs work in a recoverable goroutine that emits the
// Running and terminal status/result events. The returned snapshot mirrors the
// job as it was at submission time (status JobPending). runJob owns no
// pre-validation: callers must validate inputs before calling it.
func (o *Orchestrator) runJob(ctx context.Context, job *Job, work jobWork) *Job {
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
	snapshot := cloneJob(job)

	o.jobs.wg.Add(1)
	go func() {
		// Defers run last-registered-first. Order matters:
		//  1. recover (registered last, runs first) turns a panic in the job
		//     body into a JobFailed transition instead of crashing the process.
		//  2. finish stamps the end time and releases the cancel func.
		//  3. wg.Done (registered first, runs last) releases the shutdown wait
		//     only after the terminal status has already been recorded.
		defer o.jobs.wg.Done()
		defer o.jobs.finish(jobID)
		defer func() {
			if r := recover(); r != nil {
				o.setJobStatus(jobID, JobFailed, fmt.Errorf("job panicked: %v", r))
			}
		}()

		o.setJobStatus(jobID, JobRunning, nil)

		commit, err := work(jobCtx)
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
			commit()
		}
	}()

	return snapshot
}
