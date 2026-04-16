package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/webclient"
)

// mokuAnalyzer is the Moku-native Analyzer backend. It composes a
// webclient.WebClient (for fetching the target) with an assessor.Assessor
// (for producing evidence) and adapts the result into the unified
// industry-shaped Finding model. Externally it is indistinguishable from
// a Burp or ZAP adapter — same async contract, same result shape.
type mokuAnalyzer struct {
	cfg      MokuConfig
	poll     PollOptions
	wc       webclient.WebClient
	assessor assessor.Assessor
	logger   logging.Logger

	registry   *jobRegistry
	lifeCtx    context.Context
	lifeCancel context.CancelFunc
	closeOnce  sync.Once
}

// NewMokuAnalyzer constructs the Moku-native backend. Returns an error when
// any required dependency is nil. Callers should usually go through
// NewAnalyzer (factory) rather than calling this directly.
func NewMokuAnalyzer(cfg MokuConfig, poll PollOptions, wc webclient.WebClient, a assessor.Assessor, logger logging.Logger) (Analyzer, error) {
	if wc == nil {
		return nil, errors.New("moku analyzer: webclient is nil")
	}
	if a == nil {
		return nil, errors.New("moku analyzer: assessor is nil")
	}
	if logger == nil {
		return nil, errors.New("moku analyzer: logger is nil")
	}

	componentLogger := logger.With(logging.Field{Key: "component", Value: "moku_analyzer"})
	lifeCtx, cancel := context.WithCancel(context.Background())

	m := &mokuAnalyzer{
		cfg:        cfg,
		poll:       poll,
		wc:         wc,
		assessor:   a,
		logger:     componentLogger,
		registry:   newJobRegistry(),
		lifeCtx:    lifeCtx,
		lifeCancel: cancel,
	}
	if cfg.JobRetention > 0 {
		go m.cleanupLoop(cfg.JobRetention)
	}
	componentLogger.Info("moku analyzer constructed")
	return m, nil
}

func (m *mokuAnalyzer) Name() Backend { return BackendMoku }

func (m *mokuAnalyzer) Capabilities() Capabilities {
	return Capabilities{
		Async:               true,
		SupportsAuth:        false,
		SupportsScope:       false,
		SupportsScanProfile: true,
		Version:             "moku-0.1.0",
	}
}

func (m *mokuAnalyzer) SubmitScan(ctx context.Context, req *ScanRequest) (string, error) {
	if req == nil {
		return "", errors.New("SubmitScan: nil request")
	}
	if req.URL == "" {
		return "", errors.New("SubmitScan: empty URL")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	jobID := uuid.NewString()
	submittedAt := time.Now()
	m.registry.put(jobID, &ScanResult{
		JobID:       jobID,
		Backend:     BackendMoku,
		Status:      StatusPending,
		URL:         req.URL,
		SubmittedAt: submittedAt,
		Findings:    []Finding{},
		Summary:     &ScanSummary{},
	})

	// Run the pipeline under the analyzer's lifecycle context, not the
	// caller's — the caller may discard ctx after SubmitScan returns.
	// Close() cancels lifeCtx and tears the pipeline down.
	go m.runPipeline(jobID, submittedAt, req)

	return jobID, nil
}

func (m *mokuAnalyzer) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	if jobID == "" {
		return nil, errors.New("GetScan: empty job ID")
	}
	return m.registry.get(jobID)
}

func (m *mokuAnalyzer) ScanAndWait(ctx context.Context, req *ScanRequest, opts PollOptions) (*ScanResult, error) {
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
		opts = m.poll
	}
	jobID, err := m.SubmitScan(ctx, req)
	if err != nil {
		return nil, err
	}
	return pollUntilDone(ctx, m, jobID, opts)
}

func (m *mokuAnalyzer) Health(ctx context.Context) (string, error) {
	if m.wc == nil || m.assessor == nil {
		return "unavailable", errors.New("missing dependencies")
	}
	return "ok", nil
}

func (m *mokuAnalyzer) Close() error {
	m.closeOnce.Do(func() {
		if m.lifeCancel != nil {
			m.lifeCancel()
		}
	})
	return nil
}

// runPipeline executes the fetch → build-snapshot → score → map-findings
// pipeline for one scan. Runs in a goroutine spawned by SubmitScan. All state
// transitions are published through the jobRegistry so GetScan / ScanAndWait
// can observe progress.
func (m *mokuAnalyzer) runPipeline(jobID string, submittedAt time.Time, req *ScanRequest) {
	ctx := m.lifeCtx

	m.registry.put(jobID, &ScanResult{
		JobID:       jobID,
		Backend:     BackendMoku,
		Status:      StatusRunning,
		URL:         req.URL,
		SubmittedAt: submittedAt,
		Progress:    &ScanProgress{Percent: 10, Phase: "fetching"},
		Findings:    []Finding{},
		Summary:     &ScanSummary{},
	})

	resp, err := m.wc.Get(ctx, req.URL)
	if err != nil {
		m.storeFailure(jobID, req.URL, submittedAt, fmt.Errorf("fetch: %w", err))
		return
	}

	m.registry.put(jobID, &ScanResult{
		JobID:       jobID,
		Backend:     BackendMoku,
		Status:      StatusRunning,
		URL:         req.URL,
		SubmittedAt: submittedAt,
		Progress:    &ScanProgress{Percent: 60, Phase: "scoring"},
		Findings:    []Finding{},
		Summary:     &ScanSummary{},
	})

	snapshot := buildSnapshot(resp, req.URL, jobID)
	scoreRes, err := m.assessor.ScoreSnapshot(ctx, snapshot, jobID)
	if err != nil {
		m.storeFailure(jobID, req.URL, submittedAt, fmt.Errorf("score: %w", err))
		return
	}

	findings := findingsFromScoreResult(scoreRes)
	summary := computeSummary(findings)
	rawData := map[string]any{}
	if scoreRes != nil {
		rawData["moku.exposure_score"] = scoreRes.ExposureScore
		rawData["moku.hardening_score"] = scoreRes.HardeningScore
		if scoreRes.AttackSurface != nil {
			rawData["moku.attack_surface"] = scoreRes.AttackSurface
		}
	}

	completedAt := time.Now()
	m.registry.put(jobID, &ScanResult{
		JobID:       jobID,
		Backend:     BackendMoku,
		Status:      StatusCompleted,
		URL:         req.URL,
		SubmittedAt: submittedAt,
		CompletedAt: &completedAt,
		Findings:    findings,
		Summary:     summary,
		RawData:     rawData,
	})
}

// storeFailure writes a terminal StatusFailed ScanResult to the registry.
// Extracted to keep runPipeline's error branches terse.
func (m *mokuAnalyzer) storeFailure(jobID, url string, submittedAt time.Time, err error) {
	completedAt := time.Now()
	m.registry.put(jobID, &ScanResult{
		JobID:       jobID,
		Backend:     BackendMoku,
		Status:      StatusFailed,
		URL:         url,
		SubmittedAt: submittedAt,
		CompletedAt: &completedAt,
		Error:       err.Error(),
		Findings:    []Finding{},
		Summary:     &ScanSummary{},
	})
}

// cleanupLoop periodically evicts terminal registry entries older than the
// configured retention window. Runs under the analyzer's lifecycle context
// and exits on Close.
func (m *mokuAnalyzer) cleanupLoop(retention time.Duration) {
	if retention <= 0 {
		return
	}
	tickerPeriod := max(retention/2, time.Second)
	ticker := time.NewTicker(tickerPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-m.lifeCtx.Done():
			return
		case <-ticker.C:
			m.registry.sweepOlderThan(retention)
		}
	}
}

// buildSnapshot translates a webclient.Response into the tracker/models.Snapshot
// shape expected by assessor.Assessor. Pure function so it is trivially
// unit-testable.
func buildSnapshot(resp *webclient.Response, url, versionID string) *models.Snapshot {
	if resp == nil {
		return &models.Snapshot{
			ID:        uuid.NewString(),
			VersionID: versionID,
			URL:       url,
			CreatedAt: time.Now(),
		}
	}
	headers := make(map[string][]string, len(resp.Headers))
	for k, v := range resp.Headers {
		headers[k] = append([]string(nil), v...)
	}
	return &models.Snapshot{
		ID:         uuid.NewString(),
		VersionID:  versionID,
		StatusCode: resp.StatusCode,
		URL:        url,
		Body:       resp.Body,
		Headers:    headers,
		CreatedAt:  resp.FetchedAt,
	}
}

// findingsFromScoreResult converts Moku-internal assessor evidence into the
// unified Finding model. Pure function — no I/O, no time dependencies. Unit
// tested indirectly via the moku_analyzer_test pipeline tests and directly
// through the return shape.
func findingsFromScoreResult(sr *assessor.ScoreResult) []Finding {
	if sr == nil {
		return []Finding{}
	}
	findings := make([]Finding, 0, len(sr.Evidence))
	for _, ev := range sr.Evidence {
		findings = append(findings, evidenceToFinding(ev))
	}
	return findings
}

// evidenceToFinding is the single-item mapping rule described in the plan.
// Rich Moku-specific fields (AttackSurface, ExposureScore, HardeningScore)
// are NOT included here — they are stashed on ScanResult.RawData by the
// caller.
func evidenceToFinding(ev assessor.EvidenceItem) Finding {
	f := Finding{
		ID:          ev.RuleID,
		Title:       humanizeKey(ev.Key),
		Severity:    severityFromAssessor(ev.Severity),
		Confidence:  ConfidenceFirm,
		Description: ev.Description,
	}
	if f.ID == "" {
		// Fall back to EvidenceItem.ID or Key so Finding.ID is always non-empty.
		if ev.ID != "" {
			f.ID = ev.ID
		} else {
			f.ID = ev.Key
		}
	}
	if s, ok := ev.Value.(string); ok {
		f.Evidence = s
	}
	if len(ev.Locations) > 0 {
		loc := ev.Locations[0]
		switch {
		case loc.HeaderName != "":
			f.Parameter = loc.HeaderName
		case loc.CookieName != "":
			f.Parameter = loc.CookieName
		case loc.ParamName != "":
			f.Parameter = loc.ParamName
		}
		if len(ev.Locations) > 1 {
			f.RawData = map[string]any{"moku.locations": ev.Locations}
		}
	}
	if ev.Contribution != 0 {
		if f.RawData == nil {
			f.RawData = map[string]any{}
		}
		f.RawData["moku.contribution"] = ev.Contribution
	}
	return f
}

// severityFromAssessor normalizes the assessor's free-form severity string
// into the canonical Severity enum. Unknown values collapse to SeverityInfo
// to preserve the LSP contract ("every finding has a valid severity bucket").
func severityFromAssessor(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info", "informational":
		return SeverityInfo
	case "low":
		return SeverityLow
	case "medium", "moderate":
		return SeverityMedium
	case "high":
		return SeverityHigh
	case "critical":
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// humanizeKey turns an evidence key like "missing-csp" or "has_iframe" into
// a sentence-cased title ("Missing csp", "Has iframe"). Deliberately simple;
// richer mappings belong on the assessor rules themselves, not here.
func humanizeKey(key string) string {
	if key == "" {
		return ""
	}
	clean := strings.ReplaceAll(strings.ReplaceAll(key, "-", " "), "_", " ")
	if clean == "" {
		return ""
	}
	return strings.ToUpper(clean[:1]) + clean[1:]
}

// computeSummary aggregates Findings into per-severity counts. The per-severity
// counts MUST sum to Total — enforced by the contract test.
func computeSummary(findings []Finding) *ScanSummary {
	s := &ScanSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case SeverityInfo:
			s.Info++
		case SeverityLow:
			s.Low++
		case SeverityMedium:
			s.Medium++
		case SeverityHigh:
			s.High++
		case SeverityCritical:
			s.Critical++
		default:
			// severityFromAssessor collapses unknowns to Info, so this branch
			// should be unreachable — but defend against future contributors
			// who construct Findings outside the Moku pipeline.
			s.Info++
		}
	}
	return s
}
