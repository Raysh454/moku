package assessor

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// HeuristicsAssessor is a scaffolded, heuristics-driven implementation
// of the assessor contract. This file provides the type, constructor and
// method stubs so the rest of the codebase can depend on a concrete type
// without an implementation of any scoring logic yet.
type HeuristicsAssessor struct {
	cfg    *Config
	logger logging.Logger
	rules  []Rule
}

// NewHeuristicsAssessor constructs a heuristics-based assessor scaffold.
// It returns Assessor so callers can depend on the interfaces package contract.
//
// Note: this constructor requires a non-nil logger. If you prefer a no-op logger
// fallback, I can add a small noop logger implementation under internal/logging.
func NewHeuristicsAssessor(cfg *Config, rules []Rule, logger logging.Logger) (Assessor, error) {
	if cfg == nil {
		return nil, ErrNilConfig()
	}
	if logger == nil {
		return nil, errors.New("assessor: nil logger; please pass a valid logging.Logger")
	}

	// create a context-aware logger for this component and construct the instance
	l := logger.With(logging.Field{Key: "component", Value: "heuristics-assessor"})

	for i := range rules {
		if rules[i].Regex != "" {
			compiled, err := regexp.Compile(rules[i].Regex)
			if err != nil {
				logger.Warn("failed to compile regex rule", logging.Field{Key: "rule", Value: rules[i].ID}, logging.Field{Key: "err", Value: err})
				continue
			}
			rules[i].compiled = compiled
		}
	}

	inst := &HeuristicsAssessor{
		cfg:    cfg,
		logger: l,
		rules:  rules,
	}

	l.Info("heuristics assessor constructed", logging.Field{Key: "scoring_version", Value: cfg.ScoringVersion})

	return inst, nil
}

// ScoreHTML evaluates raw HTML bytes. Current scaffold: logs the call and returns a neutral result.
// TODO: implement preprocess -> feature extraction -> rule evaluation -> combine heuristics -> produce ScoreResult
func (h *HeuristicsAssessor) ScoreHTML(ctx context.Context, html []byte, source string, opts ScoreOptions) (*ScoreResult, error) {
	// defensive: if logger is nil (shouldn't happen when constructed properly), avoid panic
	if h == nil || h.logger == nil {
		// return a neutral result based on minimal defaults if we can (avoid panic in tests)
		defaultCfg := &Config{}
		now := time.Now().UTC()
		return &ScoreResult{
			Score:      0.0,
			Normalized: 0,
			Confidence: defaultCfg.DefaultConfidence,
			Version:    defaultCfg.ScoringVersion,
			Evidence: []EvidenceItem{
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
		}, nil
	}

	h.logger.Info("heuristics-assessor: ScoreHTML called", logging.Field{Key: "size_bytes", Value: len(html)})

	// For now return scaffold default result; opts is accepted for future behavior
	return h.defaultResult(), nil
}

// ScoreResponse delegates to ScoreHTML by extracting resp.Body.
// If resp is nil or has no body, returns a neutral result.
func (h *HeuristicsAssessor) ScoreResponse(ctx context.Context, resp *webclient.Response, opts ScoreOptions) (*ScoreResult, error) {

	source := ""
	if resp != nil && resp.Request != nil {
		source = resp.Request.URL
	}
	if resp == nil || len(resp.Body) == 0 {
		if h != nil && h.logger != nil {
			h.logger.Warn("heuristics-assessor: ScoreResponse called with empty body", logging.Field{Key: "source", Value: source})
		}
		return h.defaultResult(), nil
	}
	return h.ScoreHTML(ctx, resp.Body, source, opts)
}

// Close releases resources (currently a no-op) and logs lifecycle.
func (h *HeuristicsAssessor) Close() error {
	if h == nil || h.logger == nil {
		// nothing to do
		return nil
	}
	h.logger.Info("heuristics-assessor: closed")
	return nil
}

// defaultResult returns a neutral scaffold result so the assessor can be used
// safely before rules/heuristics are implemented.
func (h *HeuristicsAssessor) defaultResult() *ScoreResult {
	now := time.Now().UTC()
	// defensive: if cfg is missing, use small defaults
	conf := 0.0
	ver := ""
	if h != nil && h.cfg != nil {
		conf = h.cfg.DefaultConfidence
		ver = h.cfg.ScoringVersion
	}
	return &ScoreResult{
		Score:      0.0,
		Normalized: 0,
		Confidence: conf,
		Version:    ver,
		Evidence: []EvidenceItem{
			{
				Key:         "no-evidence",
				Severity:    "low",
				Description: "no scoring rules ran (scaffold default)",
				RuleID:      "scaffold:no_rules",
			},
		},
		MatchedRules: []string{},
		RawFeatures:  map[string]float64{},
		Timestamp:    now,
	}
}

// ErrNilConfig returns a small typed error for missing config.
func ErrNilConfig() error {
	return errors.New("assessor: nil config")
}
