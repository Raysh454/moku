package analyzer

import (
	"context"
	"errors"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// burpAnalyzer is the Burp Suite Enterprise REST adapter. It wraps the
// commercial Burp Enterprise REST API:
//
//   - POST /v0.1/scan              — submit a scan (returns task_id)
//   - GET  /v0.1/scan/{task_id}    — poll status, retrieve issues on completion
//
// Findings on the wire carry name, severity (Info/Low/Medium/High),
// confidence (Certain/Firm/Tentative), CWE, host/path, evidence,
// issue_background, remediation_background — the full common-denominator
// shape this adapter maps into the unified Finding model.
//
// Scaffolding status: the struct, constructor, Name/Capabilities/Close,
// and factory wiring exist so a downstream plan can land the REST pipeline
// (request/response DTOs, httptest stub server, burpIssueToFinding mapping)
// without touching any package boundaries. SubmitScan / GetScan /
// ScanAndWait / Health currently return errBurpNotImplemented.
type burpAnalyzer struct {
	cfg        BurpConfig
	poll       PollOptions
	httpClient webclient.WebClient
	logger     logging.Logger
}

// NewBurpAnalyzer constructs the Burp adapter scaffold. The scaffold still
// validates dependencies so the eventual implementation cannot be constructed
// with missing collaborators — if the REST pipeline lands later, it inherits
// a valid httpClient, non-empty BaseURL, and a component-scoped logger.
func NewBurpAnalyzer(cfg BurpConfig, poll PollOptions, httpClient webclient.WebClient, logger logging.Logger) (Analyzer, error) {
	if logger == nil {
		return nil, errors.New("burp analyzer: logger is nil")
	}
	if httpClient == nil {
		return nil, errors.New("burp analyzer: httpClient is nil")
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("burp analyzer: BurpConfig.BaseURL is required")
	}
	componentLogger := logger.With(logging.Field{Key: "component", Value: "burp_analyzer"})
	componentLogger.Info("burp analyzer constructed (scaffold; REST pipeline not yet implemented)")
	return &burpAnalyzer{
		cfg:        cfg,
		poll:       poll,
		httpClient: httpClient,
		logger:     componentLogger,
	}, nil
}

// errBurpNotImplemented signals that the Burp REST pipeline has not been
// wired yet. Distinguished from input-validation errors so callers can detect
// the scaffold explicitly (e.g. via errors.Is in downstream tests).
var errBurpNotImplemented = errors.New("burp analyzer: REST pipeline not yet implemented")

func (b *burpAnalyzer) Name() Backend { return BackendBurp }

// Capabilities reports what the Burp adapter WILL support once implemented.
// The flags reflect Burp Enterprise's native feature set: API-key auth,
// scope rules via include/exclude arrays, scan_configurations presets mapped
// from ScanProfile.
func (b *burpAnalyzer) Capabilities() Capabilities {
	return Capabilities{
		Async:               true,
		SupportsAuth:        true,
		SupportsScope:       true,
		SupportsScanProfile: true,
		Version:             "burp-scaffold",
	}
}

func (b *burpAnalyzer) SubmitScan(ctx context.Context, req *ScanRequest) (string, error) {
	if req == nil {
		return "", errors.New("SubmitScan: nil request")
	}
	if req.URL == "" {
		return "", errors.New("SubmitScan: empty URL")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// TODO: build burpScanRequest DTO from req, POST to {BaseURL}/v0.1/scan
	//       via b.httpClient, parse Location header (or body) for task_id,
	//       return task_id as the job ID.
	return "", errBurpNotImplemented
}

func (b *burpAnalyzer) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	if jobID == "" {
		return nil, errors.New("GetScan: empty job ID")
	}
	// TODO: GET {BaseURL}/v0.1/scan/{jobID}, decode burpScanResponse,
	//       map issues via burpIssueToFinding, compute Summary, populate
	//       ScanResult with Status derived from Burp's scan_status field.
	return nil, errBurpNotImplemented
}

func (b *burpAnalyzer) ScanAndWait(ctx context.Context, req *ScanRequest, opts PollOptions) (*ScanResult, error) {
	if req == nil {
		return nil, errors.New("ScanAndWait: nil request")
	}
	if req.URL == "" {
		return nil, errors.New("ScanAndWait: empty URL")
	}
	// Once SubmitScan/GetScan are implemented this becomes:
	//   jobID, err := b.SubmitScan(ctx, req)
	//   if err != nil { return nil, err }
	//   return pollUntilDone(ctx, b, jobID, opts)
	return nil, errBurpNotImplemented
}

// Health will eventually probe the Burp REST API. Burp exposes no formal
// health endpoint, so the real implementation GETs {BaseURL}/v0.1/scan and
// treats a 401 (unauthorized) as "up" — anything else indicates an issue.
func (b *burpAnalyzer) Health(ctx context.Context) (string, error) {
	return "unavailable", errBurpNotImplemented
}

// Close is a no-op because the scaffold holds no resources. The eventual
// implementation may release an HTTP keepalive pool here.
func (b *burpAnalyzer) Close() error { return nil }
