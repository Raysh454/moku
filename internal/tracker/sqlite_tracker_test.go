package tracker_test

import (
	"context"
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/webclient"
	"testing"
	"time"
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

func TestNewSQLiteTracker(t *testing.T) {
	logger := logging.NewStdoutLogger("Tracker-test")
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5, ScoreOpts: assessor.ScoreOptions{RequestLocations: true, Timeout: 15 * time.Second}}

	a, err := assessor.NewHeuristicsAssessor(cfg, SecurityRules, logger)
	if err != nil {
		t.Fatalf("Failed to create HeuristicsAssessor: %v", err)
	}

	tr, err := tracker.NewSQLiteTracker(logger, a, &tracker.Config{StoragePath: "/tmp/moku"})
	if err != nil {
		t.Fatalf("Failed to create SQLiteTracker: %v", err)
	}
	defer tr.Close()

	wc, _ := webclient.NewWebClient(&app.Config{WebClientBackend: "nethttp"}, logger)
	spider := enumerator.NewSpider(1, wc, logger)

	targets, err := spider.Enumerate(context.Background(), "https://dsu.edu.pk")
	if err != nil {
		t.Fatalf("Spider enumeration failed: %v", err)
	}

	fcher, err := fetcher.New(3, 1024, tr, wc, logger, &assessor.ScoreOptions{RequestLocations: true, Timeout: 15 * time.Second})
	if err != nil {
		t.Fatalf("Failed to create fetcher: %v", err)
	}

	fcher.Fetch(context.Background(), targets)
}
