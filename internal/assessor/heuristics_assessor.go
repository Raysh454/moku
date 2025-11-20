package assessor

import (
	"context"
	"errors"
	"time"

	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

// HeuristicsAssessor is a scaffolded, heuristics-driven implementation
// of the assessor contract. This file provides the type, constructor and
// method stubs so the rest of the codebase can depend on a concrete type
// without an implementation of any scoring logic yet.
type HeuristicsAssessor struct {
	cfg    *Config
	logger interfaces.Logger
}

// NewHeuristicsAssessor constructs a heuristics-based assessor scaffold.
// It returns interfaces.Assessor so callers can depend on the interfaces package contract.
//
// Note: this constructor requires a non-nil logger. If you prefer a no-op logger
// fallback, I can add a small noop logger implementation under internal/logging.
func NewHeuristicsAssessor(cfg *Config, logger interfaces.Logger) (interfaces.Assessor, error) {
	if cfg == nil {
		return nil, ErrNilConfig()
	}
	if logger == nil {
		return nil, errors.New("assessor: nil logger; please pass a valid interfaces.Logger")
	}
	l := logger.With(interfaces.Field{Key: "component", Value: "heuristics-assessor"})
	l.Info("heuristics assessor constructed", interfaces.Field{Key: "scoring_version", Value: cfg.ScoringVersion})

	return &HeuristicsAssessor{
		cfg:    cfg,
		logger: l,
	}, nil
}

// ScoreHTML evaluates raw HTML bytes. Current scaffold: logs the call and returns a neutral result.
// TODO: implement preprocess -> feature extraction -> rule evaluation -> combine heuristics -> produce ScoreResult
func (h *HeuristicsAssessor) ScoreHTML(ctx context.Context, html []byte, source string) (*model.ScoreResult, error) {
	h.logger.Info("heuristics-assessor: ScoreHTML called", interfaces.Field{Key: "source", Value: source}, interfaces.Field{Key: "size_bytes", Value: len(html)})
	return h.defaultResult(source), nil
}

// ScoreResponse delegates to ScoreHTML by extracting resp.Body.
// If resp is nil or has no body, returns a neutral result.
func (h *HeuristicsAssessor) ScoreResponse(ctx context.Context, resp *model.Response) (*model.ScoreResult, error) {
	source := ""
	if resp != nil {
		source = resp.Request.URL
	}
	if resp == nil || len(resp.Body) == 0 {
		h.logger.Warn("heuristics-assessor: ScoreResponse called with empty body", interfaces.Field{Key: "source", Value: source})
		return h.defaultResult(source), nil
	}
	return h.ScoreHTML(ctx, resp.Body, source)
}

// Close releases resources (currently a no-op) and logs lifecycle.
func (h *HeuristicsAssessor) Close() error {
	h.logger.Info("heuristics-assessor: closed")
	return nil
}

// defaultResult returns a neutral scaffold result so the assessor can be used
// safely before rules/heuristics are implemented.
func (h *HeuristicsAssessor) defaultResult(source string) *model.ScoreResult {
	now := time.Now().UTC()
	return &model.ScoreResult{
		Score:      0.0,
		Normalized: 0,
		Confidence: h.cfg.DefaultConfidence,
		Version:    h.cfg.ScoringVersion,
		Evidence: []model.EvidenceItem{
			{
				Key:         "no-evidence",
				Severity:    "low",
				Description: "no scoring rules ran (scaffold default)",
				RuleID:      "scaffold:no_rules",
			},
		},
		MatchedRules: []string{},
		RawFeatures:  map[string]float64{},
		Meta: map[string]any{
			"source": source,
		},
		Timestamp: now,
	}
}

// ErrNilConfig returns a small typed error for missing config.
func ErrNilConfig() error {
	return errors.New("assessor: nil config")
}
