package analyzer

import (
	"context"
	"errors"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// zapAnalyzer is the OWASP ZAP REST adapter. It wraps ZAP's two-phase
// active-scan protocol:
//
//   - POST /JSON/spider/action/scan  — start crawl (returns scan ID)
//   - GET  /JSON/spider/view/status  — poll crawl progress
//   - POST /JSON/ascan/action/scan   — start active scan once crawl is done
//   - GET  /JSON/ascan/view/status   — poll active-scan progress
//   - GET  /JSON/core/view/alerts    — retrieve alerts on completion
//
// Alerts on the wire carry alert (name), risk (Informational/Low/Medium/High),
// confidence (Low/Medium/High/Confirmed), url, param, attack, evidence,
// description, solution, reference, cweid, wascid. The adapter maps these
// into the unified Finding model (including the WASC field most other
// scanners leave empty).
//
// Scaffolding status: the struct, constructor, Name/Capabilities/Close,
// and factory wiring exist so a downstream plan can land the two-phase state
// machine, request/response DTOs, an httptest stub server, and the
// zapAlertToFinding / zapRiskToSeverity mappings. SubmitScan / GetScan /
// ScanAndWait / Health currently return errZAPNotImplemented.
//
// Quirk worth preserving when implementing: ZAP wants its API key as a query
// parameter, not an Authorization header. The adapter's internal request
// helper should hide this from the rest of the pipeline.
type zapAnalyzer struct {
	cfg        ZAPConfig
	poll       PollOptions
	httpClient webclient.WebClient
	logger     logging.Logger
}

// NewZAPAnalyzer constructs the ZAP adapter scaffold. Validates dependencies
// so the eventual implementation inherits a valid httpClient, non-empty
// BaseURL, and a component-scoped logger.
func NewZAPAnalyzer(cfg ZAPConfig, poll PollOptions, httpClient webclient.WebClient, logger logging.Logger) (Analyzer, error) {
	if logger == nil {
		return nil, errors.New("zap analyzer: logger is nil")
	}
	if httpClient == nil {
		return nil, errors.New("zap analyzer: httpClient is nil")
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("zap analyzer: ZAPConfig.BaseURL is required")
	}
	componentLogger := logger.With(logging.Field{Key: "component", Value: "zap_analyzer"})
	componentLogger.Info("zap analyzer constructed (scaffold; REST pipeline not yet implemented)")
	return &zapAnalyzer{
		cfg:        cfg,
		poll:       poll,
		httpClient: httpClient,
		logger:     componentLogger,
	}, nil
}

// errZAPNotImplemented signals that the ZAP REST pipeline has not been wired
// yet. Distinguished from input-validation errors so callers can detect the
// scaffold explicitly.
var errZAPNotImplemented = errors.New("zap analyzer: REST pipeline not yet implemented")

func (z *zapAnalyzer) Name() Backend { return BackendZAP }

// Capabilities reports what the ZAP adapter WILL support once implemented.
// ZAP supports context-based authentication, scope rules via include/exclude
// regex, and ascan policy selection — all honored once the pipeline lands.
func (z *zapAnalyzer) Capabilities() Capabilities {
	return Capabilities{
		Async:               true,
		SupportsAuth:        true,
		SupportsScope:       true,
		SupportsScanProfile: true,
		Version:             "zap-scaffold",
	}
}

func (z *zapAnalyzer) SubmitScan(ctx context.Context, req *ScanRequest) (string, error) {
	if req == nil {
		return "", errors.New("SubmitScan: nil request")
	}
	if req.URL == "" {
		return "", errors.New("SubmitScan: empty URL")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// TODO: kick off the spider phase via POST /JSON/spider/action/scan,
	//       return a compound ID like "spider:{sid}|ascan:" so GetScan can
	//       advance through the state machine on subsequent polls.
	return "", errZAPNotImplemented
}

func (z *zapAnalyzer) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	if jobID == "" {
		return nil, errors.New("GetScan: empty job ID")
	}
	// TODO: parse the compound ID, walk the spider→ascan→alerts state
	//       machine, and map alerts via zapAlertToFinding. ScanProgress.Phase
	//       should expose which phase is active ("crawling" / "active-scan"
	//       / "alerting") so progress streams look sensible in the UI.
	return nil, errZAPNotImplemented
}

func (z *zapAnalyzer) ScanAndWait(ctx context.Context, req *ScanRequest, opts PollOptions) (*ScanResult, error) {
	if req == nil {
		return nil, errors.New("ScanAndWait: nil request")
	}
	if req.URL == "" {
		return nil, errors.New("ScanAndWait: empty URL")
	}
	// Once SubmitScan/GetScan are implemented this becomes:
	//   jobID, err := z.SubmitScan(ctx, req)
	//   if err != nil { return nil, err }
	//   return pollUntilDone(ctx, z, jobID, opts)
	return nil, errZAPNotImplemented
}

// Health will eventually GET {BaseURL}/JSON/core/view/version to confirm the
// ZAP daemon is reachable and report the running version.
func (z *zapAnalyzer) Health(ctx context.Context) (string, error) {
	return "unavailable", errZAPNotImplemented
}

// Close is a no-op for the scaffold. The eventual implementation may release
// an HTTP keepalive pool here.
func (z *zapAnalyzer) Close() error { return nil }
