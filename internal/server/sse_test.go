package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/testutil"
)

func TestServer_JobEventsSSE(t *testing.T) {
	// Global timeout for the test to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := Config{
		AppConfig: &app.Config{StorageRoot: t.TempDir()},
		Logger:    &testutil.DummyLogger{},
	}
	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer s.Close()

	ts := httptest.NewServer(s.router)
	defer ts.Close()

	orch := s.Orchestrator()
	t.Logf("Server started at %s", ts.URL)

	// 1. Setup project and website
	if _, err := orch.CreateProject(ctx, "p1", "Project 1", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := orch.CreateWebsite(ctx, "p1", "s1", "https://example.com"); err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	t.Log("Project and website created")

	// 2. Establish SSE connection
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/jobs/events?project=p1", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("SSE connection failed: %v", err)
	}
	defer resp.Body.Close()
	t.Log("SSE connection established")

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	// 3. Start a job to trigger events
	job, err := orch.StartFetchJob(ctx, "p1", "s1", "new", 1, nil)
	if err != nil {
		t.Fatalf("StartFetchJob: %v", err)
	}
	t.Logf("Job %s started", job.ID)

	// 4. Read SSE stream with timeout per line
	reader := bufio.NewReader(resp.Body)

	readSSE := func() (app.JobEvent, error) {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return app.JobEvent{}, err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				var ev app.JobEvent
				data := strings.TrimPrefix(line, "data: ")
				if err := json.Unmarshal([]byte(data), &ev); err != nil {
					return app.JobEvent{}, err
				}
				return ev, nil
			}
		}
	}

	// Expect the initial event
	t.Log("Waiting for initial event...")
	ev, err := readSSE()
	if err != nil {
		t.Fatalf("failed to read initial event: %v", err)
	}
	t.Logf("Received event: %s", ev.Status)
	if ev.JobID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, ev.JobID)
	}

	// 5. Test cancellation event
	orch.CancelJob(job.ID)
	t.Log("Job canceled, waiting for event...")

	// Should see a canceled event
	found := false
	for i := 0; i < 10; i++ {
		ev, err = readSSE()
		if err != nil {
			break
		}
		t.Logf("Received event in loop: %s", ev.Status)
		if ev.JobID == job.ID && ev.Status == app.JobCanceled {
			found = true
			break
		}
	}
	if !found {
		t.Error("did not receive canceled event")
	}

	// 6. Test filtering
	t.Log("Testing filtering...")
	req2, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/jobs/events?project=other", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("SSE connection 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	reader2 := bufio.NewReader(resp2.Body)

	// Create another job in p1
	job2, err := orch.StartFetchJob(ctx, "p1", "s1", "new", 1, nil)
	if err != nil {
		t.Fatalf("StartFetchJob 2: %v", err)
	}
	t.Logf("Job 2 %s started", job2.ID)

	// reader should see it, reader2 should NOT
	ev, err = readSSE()
	if err != nil {
		t.Fatalf("reader 1 failed to read second job event: %v", err)
	}
	t.Logf("Reader 1 received job 2 event: %s", ev.Status)

	// reader2 should have nothing
	done := make(chan struct{})
	go func() {
		// This will block until connection closes or we read something
		line, _ := reader2.ReadString('\n')
		if strings.HasPrefix(line, "data: ") {
			close(done)
		}
	}()

	select {
	case <-done:
		t.Error("reader 2 received event that should have been filtered out")
	case <-time.After(500 * time.Millisecond):
		t.Log("Reader 2 correctly filtered out event")
	}
}
