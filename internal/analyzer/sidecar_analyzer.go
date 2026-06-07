package analyzer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/raysh454/moku/internal/logging"
)

// sidecarAnalyzer is the Go-side client for the Python analyzer sidecar
// (services/analyzer/). One sidecarAnalyzer instance is bound to a single
// adapter name (e.g. "builtin", "nuclei", "nikto", "shodan", "virustotal")
// and forwards SubmitScan / GetScan / Health calls over HTTP to the sidecar.
//
// All adapter-specific scan logic lives inside the sidecar; the Go side is a
// thin client. The Backend identity reported by Name() is whatever the caller
// configured (BackendDAST, BackendNuclei, ...), while the adapter field is
// what gets serialized into the /scan body so the sidecar can dispatch.
//
// sidecarAnalyzer owns a dedicated *http.Client built from SidecarConfig so
// its TLS / timeout posture is decoupled from the orchestrator's general
// webclient.
type sidecarAnalyzer struct {
	cfg      SidecarConfig
	poll     PollOptions
	backend  Backend
	adapter  string
	httpDoer sidecarHTTPDoer
	logger   logging.Logger
}

// sidecarHTTPDoer is the narrow capability the analyzer needs from its
// transport — exactly what *http.Client satisfies (ISP: the analyzer depends
// only on Do, not the full *http.Client surface). Concrete instances are built
// by newSidecarHTTPClient; tests exercise the request path through an
// httptest.Server.
type sidecarHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sentinel errors. Callers (notably the orchestrator) discriminate transport
// failures from in-band sidecar failures with errors.Is.
var (
	// ErrSidecarUnreachable wraps any network-layer failure (connection
	// refused, DNS failure, TLS handshake, context deadline before headers).
	// Exported so the orchestrator can surface a human-readable message.
	ErrSidecarUnreachable = errors.New("analyzer: sidecar unreachable")

	// errSidecarBadStatus is returned when the sidecar responds with an
	// HTTP status the client cannot reduce to a more specific sentinel.
	errSidecarBadStatus = errors.New("analyzer: sidecar bad status")

	// errSidecarJobUnknown is returned when GetScan receives a 404 from the
	// sidecar. Indicates an expired or never-existing job ID.
	errSidecarJobUnknown = errors.New("analyzer: sidecar job unknown")
)

const (
	sidecarTokenHeader     = "X-Moku-Token"
	sidecarContentTypeJSON = "application/json"

	// SidecarContractVersion is the wire-contract version this client expects.
	// The sidecar reports its own version on /health; a mismatch is logged so
	// operators can spot Go/Python version skew. Must match the Python
	// CONTRACT_VERSION in services/analyzer/app/models/schemas.py.
	SidecarContractVersion = "1"
)

// newSidecarAnalyzer constructs a sidecar-backed Analyzer for a specific
// adapter. Returns an error when any dependency is nil or any required
// configuration field is empty.
func newSidecarAnalyzer(
	cfg SidecarConfig,
	poll PollOptions,
	backend Backend,
	adapter string,
	logger logging.Logger,
) (*sidecarAnalyzer, error) {
	if logger == nil {
		return nil, errors.New("sidecar analyzer: logger is nil")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("sidecar analyzer: SidecarConfig.BaseURL is required")
	}
	if strings.TrimSpace(adapter) == "" {
		return nil, errors.New("sidecar analyzer: adapter name is required")
	}
	componentLogger := logger.With(
		logging.Field{Key: "component", Value: "sidecar_analyzer"},
		logging.Field{Key: "adapter", Value: adapter},
	)
	return &sidecarAnalyzer{
		cfg:      cfg,
		poll:     poll,
		backend:  backend,
		adapter:  adapter,
		httpDoer: newSidecarHTTPClient(cfg),
		logger:   componentLogger,
	}, nil
}

// newSidecarHTTPClient builds the dedicated *http.Client used for every call
// to the sidecar. Owning a private client (rather than reusing the orchestrator's
// shared webclient.WebClient transport) lets sidecar-specific knobs —
// InsecureSkipTLS for a self-signed sidecar, a tight RequestTimeout for a
// local process — live next to the SidecarConfig they describe without
// leaking into unrelated outbound traffic.
//
// RequestTimeout is intentionally NOT applied as the *http.Client's overall
// Timeout: each call already wraps the context with cfg.RequestTimeout in
// (*sidecarAnalyzer).do, which gives a deterministic deadline-exceeded error
// instead of the *http.Client's "context deadline exceeded (Client.Timeout
// exceeded while reading body)" wrapping. The HTTP client itself stays
// timeout-free so the context is the single source of truth.
func newSidecarHTTPClient(cfg SidecarConfig) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipTLS {
		// Self-signed certs are the documented use case (loopback sidecar
		// behind HTTPS); cfg requires explicit opt-in, so a linter waiver
		// here is intentional.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- opt-in via SidecarConfig.InsecureSkipTLS
	}
	return &http.Client{Transport: transport}
}

func (s *sidecarAnalyzer) Name() Backend { return s.backend }

// sidecarAdapterCapabilities is the Strategy table consulted by
// (*sidecarAnalyzer).Capabilities. Adding a new adapter is a one-line entry
// here plus a factory branch in factory.go — no switch statement to keep in
// sync. The Async, MaxConcurrentScans, and Version fields are stamped at call
// time, so the table only describes the per-adapter feature toggles that
// actually differ.
//
// These values are kept in lock-step with the Python adapters' declared
// capabilities() via the shared manifest at testdata/capabilities.json; a
// conformance test on each side fails if they drift. Only the builtin adapter
// currently honours ScanRequest.Auth (cookies + basic/bearer); the CLI/passive
// adapters ignore auth/scope/profile, so they advertise nothing extra.
var sidecarAdapterCapabilities = map[string]Capabilities{
	"builtin":    {SupportsAuth: true},
	"nuclei":     {},
	"nikto":      {},
	"shodan":     {},
	"virustotal": {},
}

// sidecarMaxConcurrentScans is the per-adapter concurrency ceiling reported to
// callers. The sidecar serialises scans per adapter, so it is 1 for every
// adapter; kept as a named constant to match the shared capabilities manifest.
const sidecarMaxConcurrentScans = 1

// Capabilities returns a static, adapter-specific snapshot. It looks the
// adapter up in sidecarAdapterCapabilities and stamps the invariant fields
// (Async, MaxConcurrentScans, Version). Unknown adapters fall back to the
// stamped defaults — safe because every adapter served by this client is async
// and single-flight by construction (enforced by an exhaustiveness test).
//
// No network call is made; this satisfies the LSP contract that Capabilities
// is side-effect free.
func (s *sidecarAnalyzer) Capabilities() Capabilities {
	caps := sidecarAdapterCapabilities[s.adapter] // zero-value when missing
	caps.Async = true
	caps.MaxConcurrentScans = sidecarMaxConcurrentScans
	caps.Version = fmt.Sprintf("sidecar-%s", s.adapter)
	return caps
}

// SubmitScan POSTs a scan request to {BaseURL}/scan and returns the sidecar's
// generated job ID. The "backend" field in the JSON body is set to s.adapter
// so the sidecar's registry can dispatch to the correct in-process adapter.
func (s *sidecarAnalyzer) SubmitScan(ctx context.Context, req *ScanRequest) (string, error) {
	if req == nil {
		return "", errors.New("SubmitScan: nil request")
	}
	if req.URL == "" {
		return "", errors.New("SubmitScan: empty URL")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	body, err := s.encodeSubmitBody(req)
	if err != nil {
		return "", fmt.Errorf("SubmitScan: encode body: %w", err)
	}

	resp, err := s.post(ctx, "/scan", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return "", s.statusError(resp, "SubmitScan")
	}

	var decoded struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("SubmitScan: decode response: %w", err)
	}
	if decoded.JobID == "" {
		return "", errors.New("SubmitScan: sidecar returned empty job_id")
	}
	return decoded.JobID, nil
}

// GetScan fetches the current state of jobID from the sidecar. A 404 is
// translated to errSidecarJobUnknown so callers can distinguish "unknown job"
// from "transport error".
func (s *sidecarAnalyzer) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	if jobID == "" {
		return nil, errors.New("GetScan: empty job ID")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resp, err := s.get(ctx, "/scan/"+jobID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result ScanResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("GetScan: decode response: %w", err)
		}
		// The wire "backend" field carries the Python adapter name
		// (e.g. "builtin", "nuclei"). Overwrite it with the Moku Backend
		// identity owned by this client so callers always see the same
		// identifier that Name() returns. This keeps Backend identity
		// decisions on the Go side rather than letting the sidecar
		// dictate them.
		result.Backend = s.backend
		return &result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("GetScan: job %q: %w", jobID, errSidecarJobUnknown)
	default:
		return nil, s.statusError(resp, "GetScan")
	}
}

// ScanAndWait submits a scan and polls until the sidecar reports a terminal
// status. Zero PollOptions falls back to the analyzer's configured defaults.
func (s *sidecarAnalyzer) ScanAndWait(ctx context.Context, req *ScanRequest, opts PollOptions) (*ScanResult, error) {
	if req == nil {
		return nil, errors.New("ScanAndWait: nil request")
	}
	if req.URL == "" {
		return nil, errors.New("ScanAndWait: empty URL")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Interval == 0 && opts.Timeout == 0 {
		opts = s.poll
	}
	jobID, err := s.SubmitScan(ctx, req)
	if err != nil {
		return nil, err
	}
	return pollUntilDone(ctx, s, jobID, opts)
}

// Health probes the sidecar's /health endpoint. Returns the sidecar-reported
// status string verbatim ("ok" / "degraded" / "unavailable") on success, or
// ("unavailable", ErrSidecarUnreachable) on any transport failure.
func (s *sidecarAnalyzer) Health(ctx context.Context) (string, error) {
	resp, err := s.get(ctx, "/health")
	if err != nil {
		return "unavailable", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "unavailable", s.statusError(resp, "Health")
	}
	var decoded struct {
		Status          string `json:"status"`
		ContractVersion string `json:"contract_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "unavailable", fmt.Errorf("Health: decode response: %w", err)
	}
	if decoded.Status == "" {
		return "unavailable", errors.New("Health: sidecar returned empty status")
	}
	s.warnOnContractMismatch(decoded.ContractVersion)
	return decoded.Status, nil
}

// warnOnContractMismatch logs a warning when the sidecar reports a wire-contract
// version different from the one this client was built against. Empty (an older
// sidecar that predates the field) is tolerated silently; only an explicit,
// differing version is flagged, so Go/Python skew surfaces in logs without
// failing the health probe.
func (s *sidecarAnalyzer) warnOnContractMismatch(reported string) {
	if reported == "" || reported == SidecarContractVersion {
		return
	}
	s.logger.Warn(
		"sidecar contract version mismatch",
		logging.Field{Key: "sidecar_contract_version", Value: reported},
		logging.Field{Key: "client_contract_version", Value: SidecarContractVersion},
	)
}

// Close is a no-op: the sidecar analyzer holds no resources of its own. The
// shared webclient is owned by the orchestrator.
func (s *sidecarAnalyzer) Close() error { return nil }

// encodeSubmitBody assembles the JSON payload sent to /scan. Marshals only
// the fields the sidecar accepts; the adapter selector is added as "backend"
// so the sidecar can dispatch.
func (s *sidecarAnalyzer) encodeSubmitBody(req *ScanRequest) ([]byte, error) {
	payload := map[string]any{
		"url":     req.URL,
		"backend": s.adapter,
	}
	if req.Profile != "" {
		payload["profile"] = string(req.Profile)
	}
	if req.Scope != nil {
		payload["scope"] = req.Scope
	}
	if req.Auth != nil {
		payload["auth"] = req.Auth
	}
	if req.MaxDuration > 0 {
		// Go's native duration string (e.g. "5m30s"); the Python sidecar
		// parses it via its _parse_go_duration helper.
		payload["max_duration"] = req.MaxDuration.String()
	}
	if len(req.RawOptions) > 0 {
		payload["raw_options"] = req.RawOptions
	}
	return json.Marshal(payload)
}

// post performs a POST request against the sidecar with the configured shared
// secret (if any) attached as X-Moku-Token. Network failures are wrapped with
// ErrSidecarUnreachable so callers can identify transport problems.
func (s *sidecarAnalyzer) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	reqCtx, cancel := s.withRequestTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.urlFor(path), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sidecar: build POST: %w", err)
	}
	req.Header.Set("Content-Type", sidecarContentTypeJSON)
	s.attachAuthHeader(req)
	return s.do(ctx, req)
}

// get performs a GET request against the sidecar.
func (s *sidecarAnalyzer) get(ctx context.Context, path string) (*http.Response, error) {
	reqCtx, cancel := s.withRequestTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, s.urlFor(path), nil)
	if err != nil {
		return nil, fmt.Errorf("sidecar: build GET: %w", err)
	}
	s.attachAuthHeader(req)
	return s.do(ctx, req)
}

// withRequestTimeout derives a per-call context bounded by cfg.RequestTimeout
// when non-zero. Returns ctx unchanged (with a no-op cancel) when the field is
// zero so callers can unconditionally defer the returned cancel function.
func (s *sidecarAnalyzer) withRequestTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.cfg.RequestTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.cfg.RequestTimeout)
}

// do dispatches the request through the analyzer's dedicated HTTP client. The
// client is owned by sidecarAnalyzer (constructed from SidecarConfig) so its
// TLS posture and timeout semantics are independent of the orchestrator's
// general webclient.
//
// Error classification: if the supplied (parent) context has been canceled or
// its deadline exceeded, that is caller-driven — the request was aborted, the
// scanner is not necessarily offline — so the context error is returned
// unwrapped, preserving errors.Is(err, context.Canceled / DeadlineExceeded).
// Any other transport failure (connection refused, DNS, TLS, or a per-call
// RequestTimeout firing while the parent context is still live) is wrapped
// with ErrSidecarUnreachable so the orchestrator can render a "scanner
// offline" message distinct from in-band sidecar failures.
func (s *sidecarAnalyzer) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	resp, err := s.httpDoer.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: %v", ErrSidecarUnreachable, err)
	}
	return resp, nil
}

// urlFor concatenates BaseURL with a leading-slash path, trimming any trailing
// slash on BaseURL so "http://host/" + "/scan" doesn't double up.
func (s *sidecarAnalyzer) urlFor(path string) string {
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	return base + path
}

// attachAuthHeader sets the shared-secret header when configured.
func (s *sidecarAnalyzer) attachAuthHeader(req *http.Request) {
	if s.cfg.SharedSecret != "" {
		req.Header.Set(sidecarTokenHeader, s.cfg.SharedSecret)
	}
}

// statusError builds a wrapped error carrying the sidecar's HTTP status and a
// short body excerpt for diagnostics. The wrapped sentinel is errSidecarBadStatus
// so callers can identify in-band sidecar failures.
func (s *sidecarAnalyzer) statusError(resp *http.Response, op string) error {
	const maxExcerpt = 256
	excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxExcerpt))
	return fmt.Errorf("%s: sidecar returned %d: %s: %w", op, resp.StatusCode, strings.TrimSpace(string(excerpt)), errSidecarBadStatus)
}
