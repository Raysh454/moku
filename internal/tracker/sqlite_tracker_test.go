package tracker_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

var SecurityRules = []assessor.Rule{
	// =========================
	// JavaScript Injection / XSS
	// =========================

	{
		ID:       "js:inline-script",
		Key:      "inline-script",
		Severity: "medium",
		Weight:   0.15,
		Selector: `script:not([src])`,
	},

	{
		ID:       "js-eval-usage",
		Key:      "eval-usage",
		Severity: "high",
		Weight:   0.4,
		Regex:    `\b(eval|Function)\s*\(`,
	},

	{
		ID:       "js-dangerous-dom",
		Key:      "dangerous-dom",
		Severity: "high",
		Weight:   0.35,
		Regex:    `(document\.write|innerHTML\s*=|outerHTML\s*=)`,
	},

	{
		ID:       "js-obfuscation",
		Key:      "js-obfuscation",
		Severity: "high",
		Weight:   0.45,
		Regex:    `(atob\(|String\.fromCharCode|unescape\()`,
	},

	// =========================
	// HTML Injection Indicators
	// =========================

	{
		ID:       "html-event-handlers",
		Key:      "event-handlers",
		Severity: "medium",
		Weight:   0.2,
		Regex:    `on(click|load|error|mouseover|submit)\s*=`,
	},

	{
		ID:       "html-iframe",
		Key:      "iframe",
		Severity: "medium",
		Weight:   0.2,
		Selector: "iframe",
	},

	{
		ID:       "html-hidden-elements",
		Key:      "hidden-elements",
		Severity: "low",
		Weight:   0.1,
		Selector: `*[style*="display:none"], *[style*="visibility:hidden"]`,
	},

	// =========================
	// Phishing & Credential Theft
	// =========================

	{
		ID:       "form-password-input",
		Key:      "password-input",
		Severity: "medium",
		Weight:   0.25,
		Selector: `input[type="password"]`,
	},

	{
		ID:       "form-external-action",
		Key:      "external-form-action",
		Severity: "high",
		Weight:   0.4,
		Regex:    `<form[^>]+action=["']https?://`,
	},

	{
		ID:       "phishing-urgency-language",
		Key:      "urgency-language",
		Severity: "high",
		Weight:   0.35,
		Regex:    `(verify your account|account suspended|immediate action required|confirm your identity)`,
	},

	// =========================
	// Malware / Exploit Indicators
	// =========================

	{
		ID:       "malware-base64-blob",
		Key:      "large-base64",
		Severity: "high",
		Weight:   0.45,
		Regex:    `[A-Za-z0-9+/]{300,}={0,2}`,
	},

	{
		ID:       "malware-drive-by-redirect",
		Key:      "drive-by-redirect",
		Severity: "critical",
		Weight:   0.6,
		Regex:    `(window\.location|document\.location)\s*=`,
	},

	{
		ID:       "malware-executable-download",
		Key:      "exe-download",
		Severity: "critical",
		Weight:   0.6,
		Regex:    `href=["'][^"']+\.(exe|scr|bat|ps1|cmd)["']`,
	},

	{
		ID:       "malware-crypto-miner",
		Key:      "crypto-miner",
		Severity: "critical",
		Weight:   0.7,
		Regex:    `(coinhive|minero|cryptonight)`,
	},

	// =========================
	// Insecure Web Practices
	// =========================

	{
		ID:       "security-inline-css",
		Key:      "inline-style",
		Severity: "low",
		Weight:   0.1,
		Selector: "*[style]",
	},

	{
		ID:       "security-missing-csp",
		Key:      "missing-csp",
		Severity: "medium",
		Weight:   0.2,
		Selector: `meta[http-equiv="Content-Security-Policy"]`,
		// NOTE: absence detection handled elsewhere
	},

	{
		ID:       "security-http-resources",
		Key:      "http-resources",
		Severity: "medium",
		Weight:   0.25,
		Regex:    `src=["']http://`,
	},

	// =========================
	// Social Engineering / Dark Patterns
	// =========================

	{
		ID:       "darkpattern-notifications",
		Key:      "notification-bait",
		Severity: "medium",
		Weight:   0.25,
		Regex:    `(enable notifications|allow notifications to continue)`,
	},

	{
		ID:       "darkpattern-fake-download",
		Key:      "fake-download",
		Severity: "high",
		Weight:   0.4,
		Regex:    `(download now|start download|your download is ready)`,
	},

	// =========================
	// Wordpress plugin vulnerabilities
	// =========================

	{
		ID:       "wp-revslider-vuln",
		Key:      "revslider-vuln",
		Severity: "critical",
		Weight:   0.7,
		Regex:    `wp-content/plugins/revslider/`,
	},

	{
		ID:       "wp-duplicator-vuln",
		Key:      "duplicator-vuln",
		Severity: "critical",
		Weight:   0.6,
		Regex:    `wp-content/plugins/duplicator/`,
	},

	{
		ID:       "wp-file-manager-vuln",
		Key:      "wp-file-manager-vuln",
		Severity: "critical",
		Weight:   0.65,
		Regex:    `wp-content/plugins/wp-file-manager/`,
	},

	{
		ID:       "wp-slider-revolution-vuln",
		Key:      "slider-revolution-vuln",
		Severity: "critical",
		Weight:   0.7,
		Regex:    `wp-content/plugins/slider-revolution/`,
	},

	{
		ID:       "wp-contact-form-7-vuln",
		Key:      "contact-form-7-vuln",
		Severity: "high",
		Weight:   0.5,
		Regex:    `wp-content/plugins/contact-form-7/`,
	},

	{
		ID:       "wp-post-grid-vuln",
		Key:      "post-grid-vuln",
		Severity: "high",
		Weight:   0.5,
		Regex:    `wp-content/plugins/the-post-grid/`,
	},
}

// localSite returns an httptest.Server that serves a small static site
// with three inter-linked HTML pages, so the spider has something to crawl
// without hitting the real internet.
func localSite(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	// We need the server URL for the links, so we create the server first
	// with a placeholder handler, then update the mux.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
			<h1>Home</h1>
			<a href="%s/about">About</a>
			<a href="%s/contact">Contact</a>
		</body></html>`, srv.URL, srv.URL)
	})

	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
			<h1>About</h1>
			<a href="%s/">Home</a>
		</body></html>`, srv.URL)
	})

	mux.HandleFunc("/contact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
			<h1>Contact</h1>
			<a href="%s/">Home</a>
		</body></html>`, srv.URL)
	})

	return srv
}

func TestNewSQLiteTracker(t *testing.T) {
	srv := localSite(t)
	defer srv.Close()

	logger := logging.NewStdoutLogger("Tracker-test")
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5, ScoreOpts: assessor.ScoreOptions{RequestLocations: true}}

	a, err := assessor.NewHeuristicsAssessor(cfg, nil, logger)
	if err != nil {
		t.Fatalf("Failed to create HeuristicsAssessor: %v", err)
	}

	siteDir := t.TempDir()
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: siteDir, ProjectID: "0xbeef"}, logger, a)
	if err != nil {
		t.Fatalf("Failed to create SQLiteTracker: %v", err)
	}
	defer tr.Close()

	wc, _ := webclient.NewWebClient(webclient.Config{Client: webclient.ClientNetHTTP}, logger)
	spider := enumerator.NewSpider(1, wc, logger)

	targets, err := spider.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("Spider enumeration failed: %v", err)
	}

	if len(targets) == 0 {
		t.Fatal("Spider should discover at least one page")
	}

	fcher, err := fetcher.New(fetcher.Config{MaxConcurrency: 3, CommitSize: 1024, ScoreTimeout: 15 * time.Second}, tr, wc, indexer.NewIndex(tr.DB(), logger, utils.CanonicalizeOptions{}), logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	fcher.Fetch(context.Background(), targets, nil)
}

// TestNewSQLiteTracker_LiveSite runs the full pipeline against the live
// dsu.edu.pk site. Skipped when -short is passed.
//
//	go test -run TestNewSQLiteTracker_LiveSite ./internal/tracker/
//	go test -short ./...   ← this will skip it
func TestNewSQLiteTracker_LiveSite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-site test in -short mode")
	}

	logger := logging.NewStdoutLogger("Tracker-test")
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5, ScoreOpts: assessor.ScoreOptions{RequestLocations: true}}

	a, err := assessor.NewHeuristicsAssessor(cfg, nil, logger)
	if err != nil {
		t.Fatalf("Failed to create HeuristicsAssessor: %v", err)
	}

	siteDir := t.TempDir()
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{StoragePath: siteDir, ProjectID: "0xbeef"}, logger, a)
	if err != nil {
		t.Fatalf("Failed to create SQLiteTracker: %v", err)
	}
	defer tr.Close()

	wc, _ := webclient.NewWebClient(webclient.Config{Client: webclient.ClientNetHTTP}, logger)
	spider := enumerator.NewSpider(1, wc, logger)

	targets, err := spider.Enumerate(context.Background(), "https://dsu.edu.pk", nil)
	if err != nil {
		t.Fatalf("Spider enumeration failed: %v", err)
	}

	if len(targets) == 0 {
		t.Fatal("Spider should discover at least one page from dsu.edu.pk")
	}

	fcher, err := fetcher.New(fetcher.Config{MaxConcurrency: 3, CommitSize: 1024, ScoreTimeout: 15 * time.Second}, tr, wc, indexer.NewIndex(tr.DB(), logger, utils.CanonicalizeOptions{}), logger)
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	fcher.Fetch(context.Background(), targets, nil)
}
