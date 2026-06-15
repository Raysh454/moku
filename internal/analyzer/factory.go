package analyzer

import (
	"fmt"

	"github.com/raysh454/moku/internal/logging"
)

// Dependencies bundles the collaborators any Analyzer backend might need.
// All backends currently route through the Python sidecar, which builds its
// own HTTP transport from SidecarConfig, so only a Logger is required.
//
// Packaging deps as one struct keeps the factory signature stable as new
// backends arrive — adding a new collaborator is an additive field change,
// not a breaking signature change.
type Dependencies struct {
	Logger logging.Logger
}

// NewAnalyzer constructs the configured Analyzer backend. Backend selection
// is driven by cfg.Backend; empty defaults to BackendMoku so operators with
// no scanner-specific configuration get the sidecar's builtin active scanner
// by default.
//
// Every backend routes through the Python sidecar (services/analyzer/); the
// adapter-name dispatch happens inside the sidecar — the Go side selects it
// via the Backend field on each ScanRequest payload.
func NewAnalyzer(cfg Config, deps Dependencies) (Analyzer, error) {
	if deps.Logger == nil {
		return nil, fmt.Errorf("analyzer: nil logger")
	}
	backend := cfg.Backend
	if backend == "" {
		backend = BackendMoku
	}
	switch backend {
	case BackendMoku, BackendDAST, BackendNuclei, BackendNikto, BackendShodan, BackendVirusTotal, BackendZAP:
		// The sidecar client builds its own *http.Client from SidecarConfig
		// (TLS/timeout posture lives next to the config), so it needs no
		// injected webclient — only the adapter name and a logger.
		adapter, err := sidecarAdapterFor(backend)
		if err != nil {
			return nil, err
		}
		return newSidecarAnalyzer(cfg.Sidecar, cfg.DefaultPoll, backend, adapter, deps.Logger)
	default:
		return nil, fmt.Errorf("analyzer: unknown backend %q (supported: moku, dast, nuclei, nikto, shodan, virustotal, zap)", backend)
	}
}

// sidecarAdapterFor maps a Moku-side Backend constant to the adapter name the
// Python sidecar uses internally. Both BackendMoku and BackendDAST map to
// "builtin" — the sidecar's built-in active scanner (XSS/SQLi/CSRF). The two
// Go constants exist so callers can use either the product name ("moku") or the
// technique name ("dast") to reach the same engine.
func sidecarAdapterFor(b Backend) (string, error) {
	switch b {
	case BackendMoku, BackendDAST:
		return "builtin", nil
	case BackendNuclei:
		return "nuclei", nil
	case BackendNikto:
		return "nikto", nil
	case BackendShodan:
		return "shodan", nil
	case BackendVirusTotal:
		return "virustotal", nil
	case BackendZAP:
		return "zap", nil
	default:
		return "", fmt.Errorf("analyzer: no sidecar adapter for backend %q", b)
	}
}
