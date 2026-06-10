package moku

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/raysh454/moku/acceptance/internal/harness"
)

const (
	jobTimeout = 90 * time.Second

	// Snapshot ordering in the tracker uses second-level timestamps, so two
	// fetches of the same website landing within the same second cannot be
	// ordered. The DSL spaces consecutive fetches apart so "latest snapshot"
	// reads are deterministic — operational knowledge specs shouldn't carry.
	fetchSpacing = 1200 * time.Millisecond
)

// Job is a started background job, observed through the same SSE stream the
// frontend uses. Hold the handle to test asynchronous behavior; or use the
// blocking When* verbs which wait for completion.
type Job struct {
	website *Website
	id      string
	stream  *harness.SSEStream
}

// WhenEnumerated runs URL discovery and blocks until the job completes
// successfully.
func (w *Website) WhenEnumerated(t *testing.T) {
	t.Helper()
	w.WhenEnumerateStarted(t).ThenSucceeds(t)
}

// WhenEnumerateStarted starts URL discovery and returns without waiting.
func (w *Website) WhenEnumerateStarted(t *testing.T) *Job {
	t.Helper()
	return w.startJob(t, "enumerate", nil)
}

// WhenFetched fetches all known endpoints and blocks until the job completes
// successfully.
func (w *Website) WhenFetched(t *testing.T) {
	t.Helper()
	w.WhenFetchStarted(t).ThenSucceeds(t)
}

// WhenFetchStarted starts a fetch job and returns without waiting — the
// escape hatch for scenarios about asynchronous behavior. Pair it with
// Job.ThenSucceeds.
func (w *Website) WhenFetchStarted(t *testing.T) *Job {
	t.Helper()
	w.spaceFetchesApart()
	return w.startJob(t, "fetch", map[string]any{"status": "*", "limit": 100})
}

func (w *Website) spaceFetchesApart() {
	if w.lastFetchStarted.IsZero() {
		w.lastFetchStarted = time.Now()
		return
	}
	if wait := fetchSpacing - time.Since(w.lastFetchStarted); wait > 0 {
		time.Sleep(wait)
	}
	w.lastFetchStarted = time.Now()
}

// startJob submits the job, then subscribes to the SSE event stream filtered
// by the job's id. The server replays the job's current status on subscribe,
// so the terminal event cannot be missed even if the job finishes before the
// stream opens.
func (w *Website) startJob(t *testing.T, kind string, body map[string]any) *Job {
	t.Helper()

	var started struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	w.server().api.MustStatus(t, http.StatusAccepted, http.MethodPost, w.path("/jobs/"+kind), body, &started)
	if started.ID == "" || started.Type != kind {
		t.Fatalf("unexpected start-%s-job response: %+v", kind, started)
	}

	streamURL := w.server().api.BaseURL() + "/jobs/events?job_id=" + url.QueryEscape(started.ID)
	stream := harness.OpenSSE(t, streamURL, jobTimeout)

	return &Job{website: w, id: started.ID, stream: stream}
}

// ThenSucceeds blocks until the job reaches a terminal status and asserts it
// finished as "done".
func (j *Job) ThenSucceeds(t *testing.T) {
	t.Helper()

	event := j.awaitTerminalEvent(t)
	if event.Status != "done" {
		t.Fatalf("job %s ended in status %q (error=%q); server logs:\n%s",
			j.id, event.Status, event.Error, j.website.server().proc.Logs())
	}
}

// ThenFails blocks until the job reaches a terminal status and asserts it
// ended as "failed" — the contract for work against an unusable target.
func (j *Job) ThenFails(t *testing.T) {
	t.Helper()

	event := j.awaitTerminalEvent(t)
	if event.Status != "failed" {
		t.Fatalf("job %s ended in status %q, want \"failed\"; server logs:\n%s",
			j.id, event.Status, j.website.server().proc.Logs())
	}
}

type jobEvent struct {
	JobID  string `json:"job_id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (j *Job) awaitTerminalEvent(t *testing.T) jobEvent {
	t.Helper()
	defer j.stream.Close()

	for {
		payload, err := j.stream.ReadEvent()
		if err != nil {
			t.Fatalf("job %s: reading SSE events: %v; server logs:\n%s",
				j.id, err, j.website.server().proc.Logs())
		}

		var event jobEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("job %s: malformed SSE event %q: %v", j.id, payload, err)
		}
		if event.JobID != j.id {
			continue
		}
		if isTerminalStatus(event.Status) {
			return event
		}
	}
}

func isTerminalStatus(status string) bool {
	switch status {
	case "done", "failed", "canceled":
		return true
	}
	return false
}

// server walks up to the owning Server; kept as a method so the navigation
// lives in one place.
func (w *Website) server() *Server {
	return w.project.server
}
