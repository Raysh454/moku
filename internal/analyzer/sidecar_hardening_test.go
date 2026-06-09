package analyzer_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/analyzer"
)

// ─── Job ID validation ─────────────────────────────────────────────────

// Job IDs are interpolated into the /scan/{id} URL path. They are
// sidecar-generated UUIDs in practice, but the client validates them anyway
// so it stays safe if ever reused with untrusted ID sources.
func TestSidecarAnalyzer_GetScan_RejectsMalformedJobIDWithoutCallingSidecar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		jobID string
	}{
		{"path_traversal", "../admin"},
		{"embedded_slash", "jobs/123"},
		{"embedded_backslash", `jobs\123`},
		{"query_metacharacter", "abc?x=1"},
		{"fragment_metacharacter", "abc#frag"},
		{"percent_encoded_traversal", "..%2Fadmin"},
		{"embedded_whitespace", "job 123"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := newSidecarEnv(t)
			a := env.newAnalyzer(t)

			_, err := a.GetScan(context.Background(), tc.jobID)

			if err == nil {
				t.Fatalf("GetScan(%q) succeeded, want validation error", tc.jobID)
			}
			if requests := env.capturedRequests(); len(requests) != 0 {
				t.Errorf(
					"sidecar received %d request(s) for malformed job ID %q; validation must happen before any I/O",
					len(requests), tc.jobID,
				)
			}
		})
	}
}

func TestSidecarAnalyzer_GetScan_AcceptsUUIDStyleJobIDs(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"` + jobID + `","backend":"builtin","status":"running","submitted_at":"2026-01-15T10:30:45.123Z","findings":[]}`))
	}
	a := env.newAnalyzer(t)

	result, err := a.GetScan(context.Background(), "3f2a9b54-7c1d-4e8f-9a2b-0c5d6e7f8a9b")

	if err != nil {
		t.Fatalf("GetScan with UUID job ID returned error: %v", err)
	}
	if result.Status != analyzer.StatusRunning {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusRunning)
	}
}

// A sidecar (or impostor) handing back a job ID the client would refuse to
// use must be rejected at submission time, not when the ID is later used.
func TestSidecarAnalyzer_SubmitScan_RejectsMalformedJobIDFromSidecar(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.submitResponse = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"../../evil"}`))
	}
	a := env.newAnalyzer(t)

	_, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com"})

	if err == nil {
		t.Fatal("SubmitScan accepted a malformed job_id from the sidecar, want error")
	}
}

// ─── Strict response decoding ──────────────────────────────────────────

// The scan contract is explicitly versioned (SidecarContractVersion); a field
// the client does not know about means Go/Python skew and must fail loudly
// instead of being silently dropped.
func TestSidecarAnalyzer_GetScan_RejectsUnknownResponseFields(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"job-1","backend":"builtin","status":"running","submitted_at":"2026-01-15T10:30:45.123Z","findings":[],"unexpected_field":true}`))
	}
	a := env.newAnalyzer(t)

	_, err := a.GetScan(context.Background(), "job-1")

	if err == nil {
		t.Fatal("GetScan decoded a response with an unknown field, want error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("error %q does not mention the unknown field", err)
	}
}

func TestSidecarAnalyzer_SubmitScan_RejectsUnknownResponseFields(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.submitResponse = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"job-1","surprise":"x"}`))
	}
	a := env.newAnalyzer(t)

	_, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com"})

	if err == nil {
		t.Fatal("SubmitScan decoded a response with an unknown field, want error")
	}
}

// The health probe is deliberately LENIENT: it must keep working against
// older and newer sidecars whose /health payload grows fields, because it is
// the endpoint operators use to diagnose exactly such skew.
func TestSidecarAnalyzer_Health_ToleratesUnknownResponseFields(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.healthResponse = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","contract_version":"1","backend":null,"adapters_available":["builtin"],"future_field":42}`))
	}
	a := env.newAnalyzer(t)

	status, err := a.Health(context.Background())

	if err != nil {
		t.Fatalf("Health returned error on a payload with extra fields: %v", err)
	}
	if status != "ok" {
		t.Errorf("Health status = %q, want \"ok\"", status)
	}
}
