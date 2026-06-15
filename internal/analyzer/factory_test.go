package analyzer_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
)

// Shared fixtures for factory tests.
func factoryValidDeps() analyzer.Dependencies {
	return analyzer.Dependencies{
		Logger: noopLogger{},
	}
}

func factoryDefaultConfig() analyzer.Config {
	return analyzer.Config{
		DefaultPoll: analyzer.PollOptions{Timeout: 1 * time.Second, Interval: 25 * time.Millisecond},
	}
}

// ─── Shared invariants ─────────────────────────────────────────────────

func TestNewAnalyzer_NilLogger_Errors(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	deps := factoryValidDeps()
	deps.Logger = nil
	if _, err := analyzer.NewAnalyzer(cfg, deps); err == nil {
		t.Error("expected error for nil logger")
	}
}

func TestNewAnalyzer_EmptyBackend_DefaultsToMoku(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig() // Backend left empty on purpose
	cfg.Sidecar = analyzer.SidecarConfig{BaseURL: "http://127.0.0.1:8181"}
	a, err := analyzer.NewAnalyzer(cfg, factoryValidDeps())
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if got := a.Name(); got != analyzer.BackendMoku {
		t.Errorf("Name() = %q, want %q", got, analyzer.BackendMoku)
	}
}

func TestNewAnalyzer_UnknownBackend_Errors(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = "definitely-not-a-backend"
	if _, err := analyzer.NewAnalyzer(cfg, factoryValidDeps()); err == nil {
		t.Error("expected error for unknown backend")
	}
}

// ─── Removed backends ──────────────────────────────────────────────────

// The native Burp scaffold was removed without ever being implemented; the
// string "burp" must now fail construction loudly like any unknown backend.
func TestNewAnalyzer_Burp_IsNoLongerASupportedBackend(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = "burp"
	if _, err := analyzer.NewAnalyzer(cfg, factoryValidDeps()); err == nil {
		t.Error("expected error: the burp backend was removed")
	}
}

// ─── Sidecar dispatch ──────────────────────────────────────────────────
//
// Each sidecar-backed Backend (DAST/Nuclei/Nikto/Shodan/VirusTotal) must be
// dispatched by NewAnalyzer to a *sidecarAnalyzer instance configured with
// the correct Python-adapter name. We verify dispatch by submitting a scan
// through the factory-built analyzer against a recording fake sidecar and
// asserting the POST body's "backend" field matches the expected adapter.

// recordingSidecar is a minimal httptest fake that captures every POST to
// /scan so factory tests can assert the wire-level adapter name without
// reaching into the sidecarAnalyzer's internals.
type recordingSidecar struct {
	server   *httptest.Server
	bodiesMu sync.Mutex
	bodies   [][]byte
}

func newRecordingSidecar(t *testing.T) *recordingSidecar {
	t.Helper()
	rs := &recordingSidecar{}
	mux := http.NewServeMux()
	mux.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		rs.bodiesMu.Lock()
		rs.bodies = append(rs.bodies, body)
		rs.bodiesMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"factory-test-job"}`))
	})
	rs.server = httptest.NewServer(mux)
	t.Cleanup(rs.server.Close)
	return rs
}

func (rs *recordingSidecar) capturedBodies() [][]byte {
	rs.bodiesMu.Lock()
	defer rs.bodiesMu.Unlock()
	out := make([][]byte, len(rs.bodies))
	copy(out, rs.bodies)
	return out
}

// assertSidecarBackendRoutes builds an Analyzer via the factory for backend,
// fires a SubmitScan, and asserts the POST body carries the expected adapter
// name. Used by the per-backend dispatch tests below.
func assertSidecarBackendRoutes(t *testing.T, backend analyzer.Backend, expectedAdapterField string) {
	t.Helper()
	rs := newRecordingSidecar(t)

	cfg := factoryDefaultConfig()
	cfg.Backend = backend
	cfg.Sidecar = analyzer.SidecarConfig{BaseURL: rs.server.URL}

	a, err := analyzer.NewAnalyzer(cfg, factoryValidDeps())
	if err != nil {
		t.Fatalf("NewAnalyzer(%s): %v", backend, err)
	}
	t.Cleanup(func() { _ = a.Close() })

	if got := a.Name(); got != backend {
		t.Errorf("Name() = %q, want %q", got, backend)
	}

	jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{
		URL: "https://example.com/",
	})
	if err != nil {
		t.Fatalf("SubmitScan(%s): %v", backend, err)
	}
	if jobID != "factory-test-job" {
		t.Errorf("SubmitScan returned %q, want %q", jobID, "factory-test-job")
	}

	bodies := rs.capturedBodies()
	if len(bodies) != 1 {
		t.Fatalf("recorded %d submit bodies; want 1", len(bodies))
	}
	wantSubstring := `"backend":"` + expectedAdapterField + `"`
	if !strings.Contains(string(bodies[0]), wantSubstring) {
		t.Errorf("submit body does not contain %s: %s", wantSubstring, string(bodies[0]))
	}
}

func TestNewAnalyzer_DAST_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendDAST, "builtin")
}

func TestNewAnalyzer_Moku_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendMoku, "builtin")
}

func TestNewAnalyzer_Nuclei_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendNuclei, "nuclei")
}

func TestNewAnalyzer_Nikto_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendNikto, "nikto")
}

func TestNewAnalyzer_Shodan_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendShodan, "shodan")
}

func TestNewAnalyzer_VirusTotal_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendVirusTotal, "virustotal")
}

// BackendZAP routes through the sidecar's "zap" adapter — the never-implemented
// native Go ZAP scaffold was removed in favor of the working Python one.
func TestNewAnalyzer_ZAP_RoutesToSidecar(t *testing.T) {
	t.Parallel()
	assertSidecarBackendRoutes(t, analyzer.BackendZAP, "zap")
}

// Sidecar backends deliberately do NOT require deps.HTTPClient: the sidecar
// client builds its own transport from SidecarConfig. A nil HTTPClient must
// still construct successfully.
func TestNewAnalyzer_SidecarBackends_DoNotRequireHTTPClient(t *testing.T) {
	t.Parallel()
	sidecarBackends := []analyzer.Backend{
		analyzer.BackendMoku,
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
		analyzer.BackendZAP,
	}
	for _, backend := range sidecarBackends {
		t.Run(string(backend), func(t *testing.T) {
			t.Parallel()
			cfg := factoryDefaultConfig()
			cfg.Backend = backend
			cfg.Sidecar = analyzer.SidecarConfig{BaseURL: "http://127.0.0.1:8181"}
			a, err := analyzer.NewAnalyzer(cfg, factoryValidDeps())
			if err != nil {
				t.Fatalf("expected %s to construct without an injected HTTP client: %v", backend, err)
			}
			t.Cleanup(func() { _ = a.Close() })
		})
	}
}

// Each sidecar backend must reject construction when SidecarConfig.BaseURL
// is empty — without a target URL the analyzer has nowhere to send /scan.
func TestNewAnalyzer_SidecarBackends_RequireBaseURL(t *testing.T) {
	t.Parallel()
	sidecarBackends := []analyzer.Backend{
		analyzer.BackendMoku,
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
		analyzer.BackendZAP,
	}
	for _, backend := range sidecarBackends {
		t.Run(string(backend), func(t *testing.T) {
			t.Parallel()
			cfg := factoryDefaultConfig()
			cfg.Backend = backend
			cfg.Sidecar = analyzer.SidecarConfig{BaseURL: ""}
			if _, err := analyzer.NewAnalyzer(cfg, factoryValidDeps()); err == nil {
				t.Errorf("expected error when %s backend has empty Sidecar.BaseURL", backend)
			}
		})
	}
}
