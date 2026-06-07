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
		Logger:     noopLogger{},
		WebClient:  cannedHappyPathWebClient(),
		Assessor:   cannedHappyPathAssessor(),
		HTTPClient: cannedHappyPathWebClient(),
	}
}

func factoryDefaultConfig() analyzer.Config {
	return analyzer.Config{
		DefaultPoll: analyzer.PollOptions{Timeout: 1 * time.Second, Interval: 25 * time.Millisecond},
		Moku:        analyzer.MokuConfig{JobRetention: 1 * time.Minute},
		Burp:        analyzer.BurpConfig{BaseURL: "https://burp.example:1337", RequestTimeout: 10 * time.Second},
		ZAP:         analyzer.ZAPConfig{BaseURL: "http://zap.example:8090", RequestTimeout: 10 * time.Second},
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

// ─── Moku dispatch ─────────────────────────────────────────────────────

func TestNewAnalyzer_Moku_RequiresWebClientAndAssessor(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendMoku

	// Missing WebClient.
	deps := factoryValidDeps()
	deps.WebClient = nil
	if _, err := analyzer.NewAnalyzer(cfg, deps); err == nil {
		t.Error("expected error when Moku backend has nil WebClient")
	}

	// Missing Assessor.
	deps = factoryValidDeps()
	deps.Assessor = nil
	if _, err := analyzer.NewAnalyzer(cfg, deps); err == nil {
		t.Error("expected error when Moku backend has nil Assessor")
	}
}

// ─── Burp dispatch ─────────────────────────────────────────────────────

func TestNewAnalyzer_Burp_ConstructsWithValidConfig(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendBurp
	a, err := analyzer.NewAnalyzer(cfg, factoryValidDeps())
	if err != nil {
		t.Fatalf("NewAnalyzer(Burp): %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if got := a.Name(); got != analyzer.BackendBurp {
		t.Errorf("Name() = %q, want %q", got, analyzer.BackendBurp)
	}
	caps := a.Capabilities()
	if !caps.Async || !caps.SupportsAuth || !caps.SupportsScope || !caps.SupportsScanProfile {
		t.Errorf("Burp Capabilities() missing expected flags: %+v", caps)
	}
}

func TestNewAnalyzer_Burp_RequiresHTTPClient(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendBurp
	deps := factoryValidDeps()
	deps.HTTPClient = nil
	if _, err := analyzer.NewAnalyzer(cfg, deps); err == nil {
		t.Error("expected error when Burp backend has nil HTTPClient")
	}
}

func TestNewAnalyzer_Burp_RequiresBaseURL(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendBurp
	cfg.Burp.BaseURL = ""
	if _, err := analyzer.NewAnalyzer(cfg, factoryValidDeps()); err == nil {
		t.Error("expected error when Burp backend has empty BaseURL")
	}
}

// ─── ZAP dispatch ──────────────────────────────────────────────────────

func TestNewAnalyzer_ZAP_ConstructsWithValidConfig(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendZAP
	a, err := analyzer.NewAnalyzer(cfg, factoryValidDeps())
	if err != nil {
		t.Fatalf("NewAnalyzer(ZAP): %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if got := a.Name(); got != analyzer.BackendZAP {
		t.Errorf("Name() = %q, want %q", got, analyzer.BackendZAP)
	}
	caps := a.Capabilities()
	if !caps.Async || !caps.SupportsAuth || !caps.SupportsScope || !caps.SupportsScanProfile {
		t.Errorf("ZAP Capabilities() missing expected flags: %+v", caps)
	}
}

func TestNewAnalyzer_ZAP_RequiresHTTPClient(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendZAP
	deps := factoryValidDeps()
	deps.HTTPClient = nil
	if _, err := analyzer.NewAnalyzer(cfg, deps); err == nil {
		t.Error("expected error when ZAP backend has nil HTTPClient")
	}
}

func TestNewAnalyzer_ZAP_RequiresBaseURL(t *testing.T) {
	t.Parallel()
	cfg := factoryDefaultConfig()
	cfg.Backend = analyzer.BackendZAP
	cfg.ZAP.BaseURL = ""
	if _, err := analyzer.NewAnalyzer(cfg, factoryValidDeps()); err == nil {
		t.Error("expected error when ZAP backend has empty BaseURL")
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

// Sidecar backends deliberately do NOT require deps.HTTPClient: the sidecar
// client builds its own transport from SidecarConfig. A nil HTTPClient must
// still construct successfully.
func TestNewAnalyzer_SidecarBackends_DoNotRequireHTTPClient(t *testing.T) {
	t.Parallel()
	sidecarBackends := []analyzer.Backend{
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
	}
	for _, backend := range sidecarBackends {
		t.Run(string(backend), func(t *testing.T) {
			t.Parallel()
			cfg := factoryDefaultConfig()
			cfg.Backend = backend
			cfg.Sidecar = analyzer.SidecarConfig{BaseURL: "http://127.0.0.1:8181"}
			deps := factoryValidDeps()
			deps.HTTPClient = nil
			a, err := analyzer.NewAnalyzer(cfg, deps)
			if err != nil {
				t.Fatalf("expected %s to construct with nil HTTPClient: %v", backend, err)
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
		analyzer.BackendDAST,
		analyzer.BackendNuclei,
		analyzer.BackendNikto,
		analyzer.BackendShodan,
		analyzer.BackendVirusTotal,
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
