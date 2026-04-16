package analyzer

import (
	"fmt"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// Dependencies bundles the collaborators any Analyzer backend might need.
// Not every backend uses every field:
//   - Moku native:  uses Logger + WebClient + Assessor.
//   - Burp / ZAP:   use Logger + HTTPClient (to talk to their REST APIs).
//
// Packaging deps as one struct keeps the factory signature stable as new
// backends arrive — adding a new collaborator is an additive field change,
// not a breaking signature change.
type Dependencies struct {
	Logger logging.Logger

	// WebClient is used by the Moku backend to fetch the scan target. May
	// be nil for backends that talk to a remote scanner daemon.
	WebClient webclient.WebClient

	// Assessor is used by the Moku backend to score the fetched snapshot.
	// May be nil for backends that produce findings remotely.
	Assessor assessor.Assessor

	// HTTPClient is used by Burp/ZAP adapters to call their REST APIs.
	// Re-using the webclient.WebClient abstraction (rather than a raw
	// net/http client) gives those adapters uniform timeouts, logging,
	// and testable fakes.
	HTTPClient webclient.WebClient
}

// NewAnalyzer constructs the configured Analyzer backend. Backend selection
// is driven by cfg.Backend; empty defaults to BackendMoku so operators with
// no scanner-specific configuration get the in-process analyzer by default.
//
// Mirrors the switch-based factory used by webclient.NewWebClient so the two
// plugin points feel identical to contributors.
func NewAnalyzer(cfg Config, deps Dependencies) (Analyzer, error) {
	if deps.Logger == nil {
		return nil, fmt.Errorf("analyzer: nil logger")
	}
	backend := cfg.Backend
	if backend == "" {
		backend = BackendMoku
	}
	switch backend {
	case BackendMoku:
		if deps.WebClient == nil || deps.Assessor == nil {
			return nil, fmt.Errorf("analyzer: moku backend requires WebClient and Assessor")
		}
		return NewMokuAnalyzer(cfg.Moku, cfg.DefaultPoll, deps.WebClient, deps.Assessor, deps.Logger)
	case BackendBurp:
		if deps.HTTPClient == nil {
			return nil, fmt.Errorf("analyzer: burp backend requires HTTPClient")
		}
		return NewBurpAnalyzer(cfg.Burp, cfg.DefaultPoll, deps.HTTPClient, deps.Logger)
	case BackendZAP:
		if deps.HTTPClient == nil {
			return nil, fmt.Errorf("analyzer: zap backend requires HTTPClient")
		}
		return NewZAPAnalyzer(cfg.ZAP, cfg.DefaultPoll, deps.HTTPClient, deps.Logger)
	default:
		return nil, fmt.Errorf("analyzer: unknown backend %q (supported: moku, burp, zap)", backend)
	}
}
