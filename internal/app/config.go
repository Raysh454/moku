package app

import (
	"log"
	"net"
	"os"
	"strconv"
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

// Env vars consulted by DefaultConfig at process start. Library callers that
// prefer explicit configuration over env-driven defaults can ignore these and
// set the fields directly on the returned Config.
const (
	// EnvAnalyzerBackend overrides AnalyzerCfg.Backend.
	EnvAnalyzerBackend = "MOKU_ANALYZER_BACKEND"

	// EnvAnalyzerHost and EnvAnalyzerPort locate the Python analyzer
	// sidecar. They are the same family the sidecar's own scripts and Make
	// targets use for its bind address, so the two processes cannot
	// disagree about where the sidecar lives.
	EnvAnalyzerHost = "MOKU_ANALYZER_HOST"
	EnvAnalyzerPort = "MOKU_ANALYZER_PORT"

	// EnvAnalyzerToken is sent to the sidecar as the X-Moku-Token header.
	// It must hold the same value the sidecar process was started with.
	EnvAnalyzerToken = "MOKU_ANALYZER_TOKEN"

	// EnvStorageRoot overrides the base path where projects are kept.
	EnvStorageRoot = "MOKU_STORAGE_ROOT"
)

const (
	defaultSidecarHost = "127.0.0.1"
	defaultSidecarPort = "8181"
	defaultStorageRoot = "~/.config/moku"
)

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

// resolveSidecarBaseURL builds the sidecar base URL from the
// EnvAnalyzerHost/EnvAnalyzerPort family. Bind-all hosts (0.0.0.0, ::) are
// mapped to loopback: they are addresses to listen on, not to dial. An
// invalid port triggers a warning log and falls back to the default so the
// process can still boot.
func resolveSidecarBaseURL() string {
	host := strings.Trim(strings.TrimSpace(os.Getenv(EnvAnalyzerHost)), "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = defaultSidecarHost
	}

	port := strings.TrimSpace(os.Getenv(EnvAnalyzerPort))
	if port == "" {
		port = defaultSidecarPort
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		log.Printf(
			"app: %s=%q is not a valid TCP port; falling back to %s",
			EnvAnalyzerPort, port, defaultSidecarPort,
		)
		port = defaultSidecarPort
	}

	return "http://" + net.JoinHostPort(host, port)
}

// resolveStorageRoot returns EnvStorageRoot when set to a non-blank value,
// otherwise the default location.
func resolveStorageRoot() string {
	if root := strings.TrimSpace(os.Getenv(EnvStorageRoot)); root != "" {
		return root
	}
	return defaultStorageRoot
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
		StorageRoot:      resolveStorageRoot(),
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
				BaseURL:        resolveSidecarBaseURL(),
				SharedSecret:   os.Getenv(EnvAnalyzerToken),
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
