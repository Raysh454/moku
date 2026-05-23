package app

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// EnvAnalyzerBackend is the env var consulted by DefaultConfig to override
// AnalyzerCfg.Backend at process start. Library callers that prefer explicit
// configuration over env-driven defaults can ignore this and set Backend
// directly on the returned Config.
const EnvAnalyzerBackend = "MOKU_ANALYZER_BACKEND"

// analyzerBackendsByName maps the lowercase string accepted in
// EnvAnalyzerBackend to the canonical analyzer.Backend constant. Centralizing
// the mapping keeps DefaultConfig free of a sprawling switch statement and
// gives tests a single source of truth.
var analyzerBackendsByName = map[string]analyzer.Backend{
	"moku":       analyzer.BackendMoku,
	"burp":       analyzer.BackendBurp,
	"zap":        analyzer.BackendZAP,
	"dast":       analyzer.BackendDAST,
	"nuclei":     analyzer.BackendNuclei,
	"nikto":      analyzer.BackendNikto,
	"shodan":     analyzer.BackendShodan,
	"virustotal": analyzer.BackendVirusTotal,
}

// resolveAnalyzerBackend returns the analyzer.Backend chosen by the
// EnvAnalyzerBackend env var. When the var is unset or empty, the default
// (BackendMoku) is returned. An unrecognized value triggers a warning log and
// falls back to BackendMoku so the process can still boot.
func resolveAnalyzerBackend() analyzer.Backend {
	raw := strings.TrimSpace(os.Getenv(EnvAnalyzerBackend))
	if raw == "" {
		return analyzer.BackendMoku
	}
	if backend, ok := analyzerBackendsByName[strings.ToLower(raw)]; ok {
		return backend
	}
	log.Printf(
		"app: %s=%q is not a recognized analyzer backend; falling back to %q",
		EnvAnalyzerBackend, raw, analyzer.BackendMoku,
	)
	return analyzer.BackendMoku
}

// Config contains a minimal set of runtime configuration options required by
// internal modules during initial development. We intentionally keep this small
// for the dev branch — add more fields later as wiring requires them.
type Config struct {
	// StorageRoot is the base path where projects are kept.
	StorageRoot string

	// A job will be deleted during cleanup if it exceeds JobRetentionTime.
	JobRetentionTime time.Duration

	// Tracker Configuration
	trackerCfg tracker.Config

	// Fetcher Configuration
	FetcherCfg fetcher.Config

	// WebClient configuration
	WebClientCfg webclient.Config

	// Analyzer (vulnerability-scanner plugin) configuration. Selects the
	// backend (Moku native / Burp / ZAP / ...) and carries per-backend
	// settings. See internal/analyzer for the interface contract.
	AnalyzerCfg analyzer.Config

	// Assessor configuration
	assessorCfg assessor.Config

	// Url Parsing Options
	urlCfg utils.CanonicalizeOptions

	// Filter configuration (global defaults for URL/response filtering)
	FilterCfg *filter.Config
}

// DefaultConfig returns a Config populated with sensible development defaults.
func DefaultConfig() *Config {
	return &Config{
		StorageRoot:      "~/.config/moku",
		JobRetentionTime: 60 * time.Minute,
		trackerCfg: tracker.Config{
			RedactSensitiveHeaders:  false,
			StoragePath:             "",    // Needs to be set! (Website Directory)
			ProjectID:               "",    // Needs to be set! (Project Identifier for website)
			ForceProjectID:          false, // Needs to be set! (Whether to overwrite existing project ID)
			ShowBenignHeaderChanges: false,
		},
		FetcherCfg: fetcher.Config{
			MaxConcurrency: 4,
			CommitSize:     32,
			ScoreTimeout:   30 * time.Second,
		},
		WebClientCfg: webclient.Config{
			Client: webclient.ClientNetHTTP,
		},
		AnalyzerCfg: analyzer.Config{
			Backend: resolveAnalyzerBackend(),
			DefaultPoll: analyzer.PollOptions{
				Timeout:       5 * time.Minute,
				Interval:      2 * time.Second,
				BackoffFactor: 1.5,
				MaxInterval:   30 * time.Second,
			},
			Moku: analyzer.MokuConfig{
				DefaultProfile: analyzer.ProfileBalanced,
				JobRetention:   60 * time.Minute,
			},
			// Sidecar holds the connection details for the Python analyzer
			// sidecar process (services/analyzer/). Used when AnalyzerCfg.Backend
			// is one of BackendDAST / BackendNuclei / BackendNikto / BackendShodan /
			// BackendVirusTotal — those routes all share the same sidecar.
			Sidecar: analyzer.SidecarConfig{
				BaseURL:        "http://127.0.0.1:8181",
				RequestTimeout: 30 * time.Second,
			},
		},
		assessorCfg: assessor.Config{
			ScoringVersion:    "v0.1.0",
			DefaultConfidence: 0.5,
			ScoreOpts: assessor.ScoreOptions{
				RequestLocations: true,
			},
		},
		urlCfg: utils.CanonicalizeOptions{
			DropTrackingParams:     false,
			StripTrailingSlash:     true,
			DefaultScheme:          "https",
			TrackingParamAllowlist: nil,
		},
		FilterCfg: filter.DefaultConfig(),
	}
}
