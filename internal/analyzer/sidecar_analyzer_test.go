package analyzer_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
)

// sidecarTestEnv bundles a fake sidecar server with helpers for setting up a
// new sidecarAnalyzer under test. Each test gets its own server so handlers
// can capture per-test state without cross-talk.
type sidecarTestEnv struct {
	t              *testing.T
	server         *httptest.Server
	requests       []recordedRequest
	requestsMu     sync.Mutex
	submitResponse func(w http.ResponseWriter, r *http.Request)
	getResponse    func(w http.ResponseWriter, r *http.Request, jobID string)
	healthResponse func(w http.ResponseWriter, r *http.Request)
}

type recordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func newSidecarEnv(t *testing.T) *sidecarTestEnv {
	t.Helper()
	env := &sidecarTestEnv{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		env.recordRequest(r)
		if env.submitResponse != nil {
			env.submitResponse(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"job-abc"}`))
	})
	mux.HandleFunc("/scan/", func(w http.ResponseWriter, r *http.Request) {
		env.recordRequest(r)
		jobID := strings.TrimPrefix(r.URL.Path, "/scan/")
		if env.getResponse != nil {
			env.getResponse(w, r, jobID)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"job not found"}`))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		env.recordRequest(r)
		if env.healthResponse != nil {
			env.healthResponse(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","backend":null,"adapters_available":["builtin"]}`))
	})
	env.server = httptest.NewServer(mux)
	t.Cleanup(env.server.Close)
	return env
}

func (e *sidecarTestEnv) recordRequest(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()
	e.requests = append(e.requests, recordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
	})
}

func (e *sidecarTestEnv) capturedRequests() []recordedRequest {
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()
	out := make([]recordedRequest, len(e.requests))
	copy(out, e.requests)
	return out
}

// newSidecarAnalyzerForTest builds an Analyzer via the public factory and
// points it at the fake sidecar. The Backend defaults to BackendDAST because
// most tests do not care which adapter is selected.
func (e *sidecarTestEnv) newAnalyzer(t *testing.T) analyzer.Analyzer {
	return e.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL: e.server.URL,
	})
}

func (e *sidecarTestEnv) newAnalyzerWith(t *testing.T, backend analyzer.Backend, sidecarCfg analyzer.SidecarConfig) analyzer.Analyzer {
	t.Helper()
	if sidecarCfg.BaseURL == "" {
		sidecarCfg.BaseURL = e.server.URL
	}
	cfg := analyzer.Config{
		Backend: backend,
		DefaultPoll: analyzer.PollOptions{
			Timeout:  2 * time.Second,
			Interval: 10 * time.Millisecond,
		},
		Sidecar: sidecarCfg,
	}
	a, err := analyzer.NewAnalyzer(cfg, analyzer.Dependencies{
		Logger: noopLogger{},
	})
	if err != nil {
		t.Fatalf("NewAnalyzer(%s): %v", backend, err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// ─── SubmitScan ────────────────────────────────────────────────────────

func TestSidecarAnalyzer_SubmitScan_SerializesBackendField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		backend       analyzer.Backend
		wantBackend   string
		wantSubstring string
	}{
		{analyzer.BackendDAST, "builtin", `"backend":"builtin"`},
		{analyzer.BackendNuclei, "nuclei", `"backend":"nuclei"`},
		{analyzer.BackendNikto, "nikto", `"backend":"nikto"`},
		{analyzer.BackendShodan, "shodan", `"backend":"shodan"`},
		{analyzer.BackendVirusTotal, "virustotal", `"backend":"virustotal"`},
		{analyzer.BackendZAP, "zap", `"backend":"zap"`},
	}

	for _, tc := range cases {
		t.Run(string(tc.backend), func(t *testing.T) {
			t.Parallel()
			env := newSidecarEnv(t)
			a := env.newAnalyzerWith(t, tc.backend, analyzer.SidecarConfig{BaseURL: env.server.URL})

			jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{
				URL:     "https://example.com/",
				Profile: analyzer.ProfileBalanced,
			})
			if err != nil {
				t.Fatalf("SubmitScan: %v", err)
			}
			if jobID != "job-abc" {
				t.Errorf("SubmitScan returned %q, want %q", jobID, "job-abc")
			}

			reqs := env.capturedRequests()
			if len(reqs) != 1 {
				t.Fatalf("captured %d requests; want 1", len(reqs))
			}
			if reqs[0].Method != http.MethodPost || reqs[0].Path != "/scan" {
				t.Errorf("request = %s %s; want POST /scan", reqs[0].Method, reqs[0].Path)
			}
			if !strings.Contains(string(reqs[0].Body), tc.wantSubstring) {
				t.Errorf("body does not contain %s: %s", tc.wantSubstring, string(reqs[0].Body))
			}
		})
	}
}

func TestSidecarAnalyzer_SubmitScan_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if _, err := a.SubmitScan(context.Background(), nil); err == nil {
		t.Error("SubmitScan(nil) returned no error; want error")
	}
}

func TestSidecarAnalyzer_SubmitScan_EmptyURL_ReturnsError(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if _, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: ""}); err == nil {
		t.Error("SubmitScan(empty URL) returned no error; want error")
	}
}

func TestSidecarAnalyzer_SubmitScan_SendsSharedSecretHeader(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL:      env.server.URL,
		SharedSecret: "super-secret-token",
	})

	if _, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"}); err != nil {
		t.Fatalf("SubmitScan: %v", err)
	}

	reqs := env.capturedRequests()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests; want 1", len(reqs))
	}
	if got := reqs[0].Headers.Get("X-Moku-Token"); got != "super-secret-token" {
		t.Errorf("X-Moku-Token = %q, want %q", got, "super-secret-token")
	}
}

func TestSidecarAnalyzer_SubmitScan_OmitsSharedSecretHeaderWhenUnset(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if _, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"}); err != nil {
		t.Fatalf("SubmitScan: %v", err)
	}

	reqs := env.capturedRequests()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests; want 1", len(reqs))
	}
	if got := reqs[0].Headers.Get("X-Moku-Token"); got != "" {
		t.Errorf("X-Moku-Token = %q, want empty", got)
	}
}

func TestSidecarAnalyzer_SubmitScan_SerializesMaxDurationAsGoString(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if _, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{
		URL:         "https://example.com/",
		MaxDuration: 5*time.Minute + 30*time.Second,
	}); err != nil {
		t.Fatalf("SubmitScan: %v", err)
	}

	reqs := env.capturedRequests()
	if len(reqs) != 1 {
		t.Fatalf("captured %d requests; want 1", len(reqs))
	}
	if !strings.Contains(string(reqs[0].Body), `"max_duration":"5m30s"`) {
		t.Errorf("body missing max_duration as go-string: %s", string(reqs[0].Body))
	}
}

// ─── GetScan ───────────────────────────────────────────────────────────

func TestSidecarAnalyzer_GetScan_404_ReturnsJobUnknownSentinel(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"job not found"}`))
	}
	a := env.newAnalyzer(t)

	_, err := a.GetScan(context.Background(), "missing-job")
	if err == nil {
		t.Fatal("GetScan(unknown) returned no error; want errSidecarJobUnknown")
	}
	if !strings.Contains(err.Error(), "missing-job") {
		t.Errorf("GetScan error does not mention job id: %v", err)
	}
}

func TestSidecarAnalyzer_GetScan_5xx_ReturnsBadStatusSentinel(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"detail":"upstream failure"}`))
	}
	a := env.newAnalyzer(t)

	_, err := a.GetScan(context.Background(), "any-job")
	if err == nil {
		t.Fatal("GetScan(5xx) returned no error; want errSidecarBadStatus")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("GetScan error does not mention status code: %v", err)
	}
}

func TestSidecarAnalyzer_GetScan_DecodesFullScanResult(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, filepath.Join("testdata", "sidecar", "scan_result_completed_with_findings.json"))
	}
	a := env.newAnalyzer(t)

	result, err := a.GetScan(context.Background(), "any-job")
	if err != nil {
		t.Fatalf("GetScan: %v", err)
	}
	if result == nil {
		t.Fatal("GetScan returned nil result with nil error")
	}
	if result.Status != analyzer.StatusCompleted {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusCompleted)
	}
	// The sidecar's wire payload reports the adapter name ("builtin"), but the
	// Go client rewrites Backend to the constructor's identity (BackendDAST)
	// so callers always see the same value Name() returns.
	if result.Backend != analyzer.BackendDAST {
		t.Errorf("Backend = %q, want %q", result.Backend, analyzer.BackendDAST)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("len(Findings) = %d, want 3", len(result.Findings))
	}
	if result.Summary == nil {
		t.Fatal("Summary is nil")
	}
	if result.Summary.Total != 3 || result.Summary.Critical != 1 || result.Summary.High != 1 || result.Summary.Low != 1 {
		t.Errorf("Summary = %+v; want Total=3 Critical=1 High=1 Low=1", result.Summary)
	}
	// Cross-check that snake_case fields round-trip into Go camelCase struct fields.
	if result.CompletedAt == nil {
		t.Error("CompletedAt is nil; expected RFC3339-decoded value")
	}
}

func TestSidecarAnalyzer_GetScan_DecodesFailedResult(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, filepath.Join("testdata", "sidecar", "scan_result_failed_with_error.json"))
	}
	a := env.newAnalyzer(t)

	result, err := a.GetScan(context.Background(), "any-job")
	if err != nil {
		t.Fatalf("GetScan: %v", err)
	}
	if result.Status != analyzer.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusFailed)
	}
	if result.Error == "" {
		t.Error("Error field is empty; want populated message for failed scan")
	}
}

func TestSidecarAnalyzer_GetScan_EmptyJobID_ReturnsError(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if _, err := a.GetScan(context.Background(), ""); err == nil {
		t.Error("GetScan(empty job ID) returned no error; want error")
	}
}

// ─── Health ────────────────────────────────────────────────────────────

func TestSidecarAnalyzer_Health_OKStatus_ReturnsOK(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	status, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if status != "ok" {
		t.Errorf("Health status = %q, want %q", status, "ok")
	}
}

func TestSidecarAnalyzer_Health_ContractVersionMismatch_StillReturnsStatus(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.healthResponse = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// A contract_version the client was NOT built against: exercises the
		// decode of the new field and the warnOnContractMismatch branch. The
		// probe must still succeed (warn, don't fail).
		_, _ = w.Write([]byte(`{"status":"ok","contract_version":"999","adapters_available":[]}`))
	}
	a := env.newAnalyzer(t)

	status, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if status != "ok" {
		t.Errorf("Health status = %q, want %q", status, "ok")
	}
}

func TestSidecarAnalyzer_Health_DegradedStatus_PropagatesString(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.healthResponse = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"degraded","backend":null,"adapters_available":[]}`))
	}
	a := env.newAnalyzer(t)

	status, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if status != "degraded" {
		t.Errorf("Health status = %q, want %q", status, "degraded")
	}
}

func TestSidecarAnalyzer_Health_NetworkError_ReturnsUnavailableAndSentinel(t *testing.T) {
	t.Parallel()
	// Construct a sidecar pointed at a closed server to force a connection failure.
	closedServer := httptest.NewServer(http.NewServeMux())
	closedServer.Close()

	cfg := analyzer.Config{
		Backend:     analyzer.BackendDAST,
		DefaultPoll: analyzer.PollOptions{Interval: 5 * time.Millisecond, Timeout: 500 * time.Millisecond},
		Sidecar:     analyzer.SidecarConfig{BaseURL: closedServer.URL},
	}
	a, err := analyzer.NewAnalyzer(cfg, analyzer.Dependencies{
		Logger: noopLogger{},
	})
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	status, err := a.Health(context.Background())
	if err == nil {
		t.Fatal("Health on closed server returned nil error")
	}
	if status != "unavailable" {
		t.Errorf("Health status = %q, want %q", status, "unavailable")
	}
	if !errors.Is(err, analyzer.ErrSidecarUnreachable) {
		t.Errorf("error %v is not ErrSidecarUnreachable", err)
	}
}

// ─── ScanAndWait ───────────────────────────────────────────────────────

func TestSidecarAnalyzer_ScanAndWait_PollsUntilCompleted(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)

	// Submit returns job id; subsequent GETs walk pending -> running -> completed.
	var pollCount int32
	env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
		n := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			_, _ = w.Write([]byte(`{
				"job_id": "job-abc",
				"backend": "builtin",
				"status": "pending",
				"url": "https://example.com/",
				"submitted_at": "2026-01-15T10:30:45Z",
				"findings": [],
				"summary": null
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"job_id": "job-abc",
				"backend": "builtin",
				"status": "running",
				"url": "https://example.com/",
				"submitted_at": "2026-01-15T10:30:45Z",
				"findings": [],
				"summary": null
			}`))
		default:
			_, _ = w.Write([]byte(`{
				"job_id": "job-abc",
				"backend": "builtin",
				"status": "completed",
				"url": "https://example.com/",
				"submitted_at": "2026-01-15T10:30:45Z",
				"completed_at": "2026-01-15T10:31:00Z",
				"findings": [],
				"summary": {"total": 0, "info": 0, "low": 0, "medium": 0, "high": 0, "critical": 0}
			}`))
		}
	}
	a := env.newAnalyzer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{
		Timeout:  2 * time.Second,
		Interval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if result.Status != analyzer.StatusCompleted {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusCompleted)
	}
	if atomic.LoadInt32(&pollCount) < 3 {
		t.Errorf("pollCount = %d, want >= 3 (pending, running, completed)", pollCount)
	}
}

// ─── Identity / lifecycle ──────────────────────────────────────────────

func TestSidecarAnalyzer_Name_ReturnsBackend(t *testing.T) {
	t.Parallel()
	cases := []analyzer.Backend{
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
	}
	for _, backend := range cases {
		t.Run(string(backend), func(t *testing.T) {
			t.Parallel()
			env := newSidecarEnv(t)
			a := env.newAnalyzerWith(t, backend, analyzer.SidecarConfig{BaseURL: env.server.URL})
			if got := a.Name(); got != backend {
				t.Errorf("Name() = %q, want %q", got, backend)
			}
		})
	}
}

func TestSidecarAnalyzer_Capabilities_StaticPerAdapter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		backend  analyzer.Backend
		expected analyzer.Capabilities
	}{
		{
			backend:  analyzer.BackendDAST,
			expected: analyzer.Capabilities{Async: true, SupportsAuth: true, MaxConcurrentScans: 1, Version: "sidecar-builtin"},
		},
		{
			backend:  analyzer.BackendNuclei,
			expected: analyzer.Capabilities{Async: true, MaxConcurrentScans: 1, Version: "sidecar-nuclei"},
		},
		{
			backend:  analyzer.BackendNikto,
			expected: analyzer.Capabilities{Async: true, MaxConcurrentScans: 1, Version: "sidecar-nikto"},
		},
		{
			backend:  analyzer.BackendShodan,
			expected: analyzer.Capabilities{Async: true, MaxConcurrentScans: 1, Version: "sidecar-shodan"},
		},
		{
			backend:  analyzer.BackendVirusTotal,
			expected: analyzer.Capabilities{Async: true, MaxConcurrentScans: 1, Version: "sidecar-virustotal"},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.backend), func(t *testing.T) {
			t.Parallel()
			env := newSidecarEnv(t)
			a := env.newAnalyzerWith(t, tc.backend, analyzer.SidecarConfig{BaseURL: env.server.URL})
			got := a.Capabilities()
			if got != tc.expected {
				t.Errorf("Capabilities() = %+v, want %+v", got, tc.expected)
			}
		})
	}
}

func TestSidecarAnalyzer_Capabilities_DoesNotPerformNetworkIO(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	_ = a.Capabilities()

	if reqs := env.capturedRequests(); len(reqs) != 0 {
		t.Errorf("Capabilities() made %d HTTP requests; want 0", len(reqs))
	}
}

func TestSidecarAnalyzer_Close_NoError(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzer(t)

	if err := a.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("second Close (must be idempotent): %v", err)
	}
}

// ─── Fixture sanity ────────────────────────────────────────────────────

// TestSidecarFixtures_DecodeCleanly is a guard that ensures the JSON files
// committed under testdata/sidecar/ are syntactically valid and structurally
// compatible with analyzer.ScanResult. If somebody edits a fixture and
// breaks it, this test fails fast with the offending file name.
func TestSidecarFixtures_DecodeCleanly(t *testing.T) {
	t.Parallel()
	names := []string{
		"scan_result_running.json",
		"scan_result_completed_with_findings.json",
		"scan_result_failed_with_error.json",
		"scan_result_multi_severity_summary.json",
		// Golden produced by the Python serializer (millisecond + 'Z' datetimes);
		// proves the Go consumer decodes the sidecar's actual output format. Kept
		// in lock-step by services/analyzer/tests/test_contract_golden.py.
		"scan_result_python_serialized.json",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("testdata", "sidecar", name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var result analyzer.ScanResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			// json.Unmarshal silently zero-fills mismatched fields, so a
			// no-error parse alone does not prove the wire contract. Assert
			// the key fields of the Python-serialized golden actually mapped,
			// so a field rename / datetime-format change on the Python side
			// fails here (genuine bidirectional guard with test_contract_golden.py).
			if name == "scan_result_python_serialized.json" {
				if result.JobID == "" {
					t.Error("JobID did not decode (field-name drift?)")
				}
				if string(result.Status) == "" {
					t.Error("Status did not decode")
				}
				if result.SubmittedAt.IsZero() {
					t.Error("SubmittedAt did not decode (datetime-format drift?)")
				}
				if len(result.Findings) != 1 || result.Findings[0].Title != "XSS" {
					t.Errorf("Findings did not decode as expected: %+v", result.Findings)
				}
			}
		})
	}
}

// ─── Contract suite for every sidecar-backed Backend ───────────────────

// runSidecarContract drives the shared analyzer contract against a sidecar
// wired with a deterministic httptest fake. The fake handles SubmitScan,
// GetScan (registers any submitted job_id as immediately-completed), and
// Health.
func runSidecarContract(t *testing.T, backend analyzer.Backend) {
	t.Helper()
	factory := func(t *testing.T) analyzer.Analyzer {
		t.Helper()
		env := newSidecarEnv(t)
		jobs := newFakeSidecarJobs()
		env.submitResponse = func(w http.ResponseWriter, r *http.Request) {
			id := jobs.create()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"` + id + `"}`))
		}
		env.getResponse = func(w http.ResponseWriter, r *http.Request, jobID string) {
			if !jobs.has(jobID) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"detail":"job not found"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			payload := `{
				"job_id": "` + jobID + `",
				"backend": "` + sidecarAdapterNameForBackend(backend) + `",
				"status": "completed",
				"url": "https://example.com/",
				"submitted_at": "2026-01-15T10:30:45Z",
				"completed_at": "2026-01-15T10:31:00Z",
				"findings": [],
				"summary": {"total": 0, "info": 0, "low": 0, "medium": 0, "high": 0, "critical": 0}
			}`
			_, _ = w.Write([]byte(payload))
		}
		return env.newAnalyzerWith(t, backend, analyzer.SidecarConfig{BaseURL: env.server.URL})
	}
	runAnalyzerContract(t, backend, factory)
}

// fakeSidecarJobs is a tiny in-memory job-id registry the contract suite uses
// to make the sidecar fake satisfy the "GetScan on unknown jobID errors" leg
// of the contract.
type fakeSidecarJobs struct {
	mu    sync.Mutex
	known map[string]struct{}
	next  int
}

func newFakeSidecarJobs() *fakeSidecarJobs {
	return &fakeSidecarJobs{known: make(map[string]struct{})}
}

func (j *fakeSidecarJobs) create() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.next++
	id := "fake-job-" + intToString(j.next)
	j.known[id] = struct{}{}
	return id
}

func (j *fakeSidecarJobs) has(id string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, ok := j.known[id]
	return ok
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}

// sidecarAdapterNameForBackend mirrors the Go-side factory's adapter-name
// dispatch so test JSON payloads identify themselves with the same value the
// real sidecar would emit.
func sidecarAdapterNameForBackend(b analyzer.Backend) string {
	switch b {
	case analyzer.BackendDAST, analyzer.BackendMoku:
		return "builtin"
	case analyzer.BackendNuclei:
		return "nuclei"
	case analyzer.BackendNikto:
		return "nikto"
	case analyzer.BackendShodan:
		return "shodan"
	case analyzer.BackendVirusTotal:
		return "virustotal"
	default:
		return string(b)
	}
}

func TestSidecarAnalyzer_Contract_DAST(t *testing.T) {
	runSidecarContract(t, analyzer.BackendDAST)
}

func TestSidecarAnalyzer_Contract_Moku(t *testing.T) {
	runSidecarContract(t, analyzer.BackendMoku)
}

func TestSidecarAnalyzer_Contract_Nuclei(t *testing.T) {
	runSidecarContract(t, analyzer.BackendNuclei)
}

func TestSidecarAnalyzer_Contract_Nikto(t *testing.T) {
	runSidecarContract(t, analyzer.BackendNikto)
}

func TestSidecarAnalyzer_Contract_Shodan(t *testing.T) {
	runSidecarContract(t, analyzer.BackendShodan)
}

func TestSidecarAnalyzer_Contract_VirusTotal(t *testing.T) {
	runSidecarContract(t, analyzer.BackendVirusTotal)
}

// ─── RequestTimeout ────────────────────────────────────────────────────

// TestSidecarAnalyzer_SubmitScan_RespectsRequestTimeout proves that a sidecar
// which exceeds SidecarConfig.RequestTimeout causes SubmitScan to return a
// deadline-exceeded error instead of blocking indefinitely. This guards
// against a regression where the field was wired to the struct but never
// consumed by the request path.
func TestSidecarAnalyzer_SubmitScan_RespectsRequestTimeout(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	// Sidecar sleeps well past the request timeout before responding.
	env.submitResponse = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"never-arrives"}`))
	}
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL:        env.server.URL,
		RequestTimeout: 25 * time.Millisecond,
	})

	start := time.Now()
	_, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("SubmitScan returned nil error; want deadline-exceeded")
	}
	if !errors.Is(err, analyzer.ErrSidecarUnreachable) {
		t.Errorf("error %v does not wrap ErrSidecarUnreachable", err)
	}
	// We don't assert on context.DeadlineExceeded specifically because the
	// transport may translate it ("context deadline exceeded" in the wrapped
	// message); the elapsed-time guard is the behavioural signal.
	if elapsed > 250*time.Millisecond {
		t.Errorf("SubmitScan took %s; expected timeout to fire near 25ms (well under the 500ms server sleep)", elapsed)
	}
}

// TestSidecarAnalyzer_SubmitScan_CallerCancelIsNotUnreachable proves that a
// caller-driven context cancellation surfaces as context.Canceled and is NOT
// misclassified as ErrSidecarUnreachable — the scanner is reachable, the
// caller simply aborted.
func TestSidecarAnalyzer_SubmitScan_CallerCancelIsNotUnreachable(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.submitResponse = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"never-arrives"}`))
	}
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL: env.server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := a.SubmitScan(ctx, &analyzer.ScanRequest{URL: "https://example.com/"})
	if err == nil {
		t.Fatal("SubmitScan returned nil error; want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v is not context.Canceled", err)
	}
	if errors.Is(err, analyzer.ErrSidecarUnreachable) {
		t.Errorf("caller cancellation must not be reported as ErrSidecarUnreachable: %v", err)
	}
}

// TestSidecarAnalyzer_SubmitScan_BuffersBodyAcrossRequestTimeoutCancel proves
// the response body survives the deferred per-call cancel(): the server flushes
// the 202 headers, then delays the body, with a generous RequestTimeout. The
// body must still decode (do() buffers it while the timeout context is live).
func TestSidecarAnalyzer_SubmitScan_BuffersBodyAcrossRequestTimeoutCancel(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.submitResponse = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"job_id":"buffered-job"}`))
	}
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL:        env.server.URL,
		RequestTimeout: 2 * time.Second, // generous; body arrives ~100ms in
	})

	jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"})
	if err != nil {
		t.Fatalf("SubmitScan returned %v; body should buffer before the timeout cancel", err)
	}
	if jobID != "buffered-job" {
		t.Errorf("job id = %q, want %q", jobID, "buffered-job")
	}
}

// TestSidecarAnalyzer_Health_RespectsRequestTimeout exercises the same timeout
// path through Health, which uses the GET helper. Ensures both verbs honor
// SidecarConfig.RequestTimeout.
func TestSidecarAnalyzer_Health_RespectsRequestTimeout(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	env.healthResponse = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL:        env.server.URL,
		RequestTimeout: 25 * time.Millisecond,
	})

	start := time.Now()
	status, err := a.Health(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Health returned nil error; want deadline-exceeded")
	}
	if status != "unavailable" {
		t.Errorf("status = %q, want %q", status, "unavailable")
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("Health took %s; expected timeout to fire near 25ms", elapsed)
	}
}

// TestSidecarAnalyzer_SubmitScan_ZeroRequestTimeout_DoesNotBound proves the
// zero-value escape hatch: when RequestTimeout is unset, the per-call context
// derived from the parent is left untouched and only the parent's deadline
// (or lack thereof) controls request lifetime. Without this property,
// operators would discover that "leave the field at its zero value" silently
// imposed a deadline.
func TestSidecarAnalyzer_SubmitScan_ZeroRequestTimeout_DoesNotBound(t *testing.T) {
	t.Parallel()
	env := newSidecarEnv(t)
	a := env.newAnalyzerWith(t, analyzer.BackendDAST, analyzer.SidecarConfig{
		BaseURL: env.server.URL,
		// RequestTimeout intentionally left zero.
	})
	jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"})
	if err != nil {
		t.Fatalf("SubmitScan: %v", err)
	}
	if jobID == "" {
		t.Fatal("SubmitScan returned empty job ID")
	}
}

// ─── InsecureSkipTLS ───────────────────────────────────────────────────

// TestSidecarAnalyzer_InsecureSkipTLS_AllowsSelfSigned proves that the field
// is actually consumed by the underlying transport. With InsecureSkipTLS=true
// the analyzer must accept the httptest TLS server's self-signed cert; with
// the default (false) the same call must fail TLS verification. Both legs
// run against the same server so any divergence is attributable to the
// config flag alone.
func TestSidecarAnalyzer_InsecureSkipTLS_AllowsSelfSigned(t *testing.T) {
	t.Parallel()
	tlsServer := newSelfSignedSidecarServer(t)

	t.Run("verify disabled -> request succeeds", func(t *testing.T) {
		t.Parallel()
		a := newAnalyzerForServer(t, tlsServer.URL, analyzer.SidecarConfig{
			BaseURL:         tlsServer.URL,
			InsecureSkipTLS: true,
		})
		status, err := a.Health(context.Background())
		if err != nil {
			t.Fatalf("Health with InsecureSkipTLS=true: %v", err)
		}
		if status != "ok" {
			t.Errorf("status = %q, want %q", status, "ok")
		}
	})

	t.Run("verify enabled -> TLS error", func(t *testing.T) {
		t.Parallel()
		a := newAnalyzerForServer(t, tlsServer.URL, analyzer.SidecarConfig{
			BaseURL:         tlsServer.URL,
			InsecureSkipTLS: false,
		})
		_, err := a.Health(context.Background())
		if err == nil {
			t.Fatal("Health with InsecureSkipTLS=false succeeded against self-signed server")
		}
		if !errors.Is(err, analyzer.ErrSidecarUnreachable) {
			t.Errorf("error %v does not wrap ErrSidecarUnreachable", err)
		}
	})
}

// newSelfSignedSidecarServer spins up an httptest.NewTLSServer that responds
// "ok" on /health. The server's certificate is self-signed by the httptest
// package so a default *http.Client rejects it.
func newSelfSignedSidecarServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newAnalyzerForServer is a small construction helper that points an analyzer
// at the given URL. It exists so the TLS tests can supply a SidecarConfig
// with InsecureSkipTLS toggled without going through the env.newAnalyzerWith
// helper (which expects an env-bound server).
func newAnalyzerForServer(t *testing.T, baseURL string, cfg analyzer.SidecarConfig) analyzer.Analyzer {
	t.Helper()
	if cfg.BaseURL == "" {
		cfg.BaseURL = baseURL
	}
	a, err := analyzer.NewAnalyzer(analyzer.Config{
		Backend: analyzer.BackendDAST,
		DefaultPoll: analyzer.PollOptions{
			Timeout:  1 * time.Second,
			Interval: 10 * time.Millisecond,
		},
		Sidecar: cfg,
	}, analyzer.Dependencies{
		Logger: noopLogger{},
	})
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// ─── Capabilities table exhaustiveness ─────────────────────────────────

// TestSidecarAnalyzer_Capabilities_EveryRoutedBackendHasEntry guards the
// Strategy table in sidecar_analyzer.go: when a new sidecar-routed Backend
// constant is added, the table must gain a matching adapter entry or the
// fallback (zero-value Capabilities) will leak into UI surfaces. We exercise
// the table via the public Capabilities() method to avoid coupling the test
// to the package-private map name.
func TestSidecarAnalyzer_Capabilities_EveryRoutedBackendHasEntry(t *testing.T) {
	t.Parallel()
	// Every Backend that the factory routes through the sidecar must publish
	// non-default Capabilities. "Non-default" here means Version is stamped
	// (sidecar-<adapter>) and Async is true.
	routed := []analyzer.Backend{
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
	}
	env := newSidecarEnv(t)
	for _, backend := range routed {
		t.Run(string(backend), func(t *testing.T) {
			t.Parallel()
			a := env.newAnalyzerWith(t, backend, analyzer.SidecarConfig{BaseURL: env.server.URL})
			caps := a.Capabilities()
			if !caps.Async {
				t.Errorf("Capabilities().Async = false for %s; sidecar adapters are always async", backend)
			}
			if !strings.HasPrefix(caps.Version, "sidecar-") {
				t.Errorf("Capabilities().Version = %q for %s; expected sidecar-<adapter>", caps.Version, backend)
			}
		})
	}
}
