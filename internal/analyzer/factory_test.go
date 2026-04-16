package analyzer_test

import (
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
