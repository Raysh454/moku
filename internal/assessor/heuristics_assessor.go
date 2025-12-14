package assessor

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/PuerkitoBio/goquery"
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

// ScoreHTML evaluates raw HTML bytes using regex and CSS selector rules.
// Efficiently populates EvidenceLocation without XPath by using byte and line offsets.
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
			MatchedRules: []Rule{},
			RawFeatures:  map[string]float64{},
			Meta: map[string]any{
				"source": source,
			},
			Timestamp: now,
		}, nil
	}

	// Prepare result scaffolding
	res := &ScoreResult{
		Score:        0.0,
		Normalized:   0,
		Confidence:   h.cfg.DefaultConfidence,
		Version:      h.cfg.ScoringVersion,
		Evidence:     []EvidenceItem{},
		MatchedRules: []Rule{},
		RawFeatures:  map[string]float64{},
		Meta:         map[string]any{"source": source},
		Timestamp:    time.Now().UTC(),
	}

	// Build compact line-start offsets and mapping helper
	lineStarts := buildLineStarts(html)
	byteToLine := func(pos int) int { return byteToLineFromStarts(lineStarts, pos) }

	// Run rules
	for _, r := range h.rules {
		if r.Regex != "" && r.compiled != nil {
			appendRegexEvidence(html, byteToLine, r, res, h.cfg, opts)
		}
		if r.Selector != "" {
			appendCSSEvidence(html, byteToLine, r, res, h.logger, h.cfg, opts)
		}
	}

	// If no evidence matched, provide a scaffold default item for downstream expectations
	if len(res.Evidence) == 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         "no-evidence",
			Severity:    "low",
			Description: "no scoring rules ran (scaffold default)",
			RuleID:      "scaffold:no_rules",
		})
	}

	// Normalize score to [0..100]
	if res.Score < 0 {
		res.Score = 0
	}
	if res.Score > 1 {
		res.Score = 1
	}
	res.Normalized = int(res.Score * 100.0)

	return res, nil
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
		MatchedRules: []Rule{},
		RawFeatures:  map[string]float64{},
		Timestamp:    now,
	}
}

// ErrNilConfig returns a small typed error for missing config.
func ErrNilConfig() error {
	return errors.New("assessor: nil config")
}

// condLocations returns locations only if requested; otherwise empty to save memory.
func condLocations(enabled bool, locs []EvidenceLocation) []EvidenceLocation {
	if !enabled {
		return nil
	}
	return locs
}

// effectiveWeight returns rule weight possibly overridden by cfg.RuleWeights.
func effectiveWeight(cfg *Config, ruleID string, def float64) float64 {
	if cfg != nil && cfg.RuleWeights != nil {
		if w, ok := cfg.RuleWeights[ruleID]; ok {
			return w
		}
	}
	return def
}

// Helpers extracted for clarity and testability

func buildLineStarts(html []byte) []int {
	starts := make([]int, 0, 256)
	starts = append(starts, 0)
	for i, b := range html {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func byteToLineFromStarts(lineStarts []int, pos int) int {
	lo, hi := 0, len(lineStarts)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		start := lineStarts[mid]
		if start == pos {
			return mid + 1
		}
		if start < pos {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func appendRegexEvidence(html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, cfg *Config, opts ScoreOptions) {
	if rule.compiled == nil {
		return
	}
	locs := []EvidenceLocation{}
	for _, m := range rule.compiled.FindAllIndex(html, -1) {
		start, end := m[0], m[1]
		ls, le := byteToLine(start), byteToLine(end)
		s, e, ls1, le1 := start, end, ls, le
		locs = append(locs, EvidenceLocation{
			ByteStart: &s,
			ByteEnd:   &e,
			LineStart: &ls1,
			LineEnd:   &le1,
		})
	}
	if len(locs) > 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         rule.Key,
			RuleID:      rule.ID,
			Severity:    rule.Severity,
			Description: "regex match",
			Locations:   condLocations(opts.RequestLocations, locs),
		})
		res.MatchedRules = append(res.MatchedRules, rule)
		res.Score += effectiveWeight(cfg, rule.ID, rule.Weight)
	}
}

func appendCSSEvidence(html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, logger logging.Logger, cfg *Config, opts ScoreOptions) {
	if rule.Selector == "" {
		return
	}
	doc, err := goqueryDocumentFromBytes(html)
	if err != nil {
		if logger != nil {
			logger.Warn("css parse failed", logging.Field{Key: "err", Value: err}, logging.Field{Key: "rule", Value: rule.ID})
		}
		return
	}
	selection := doc.Find(rule.Selector)
	if selection.Length() == 0 {
		return
	}
	locs := []EvidenceLocation{}
	selection.Each(func(i int, s *goquery.Selection) {
		outer := goqueryOuterHTML(s)
		idx := indexOf(html, []byte(outer))
		if idx >= 0 {
			start := idx
			end := idx + len(outer)
			ls, le := byteToLine(start), byteToLine(end)
			s1, e1, ls1, le1 := start, end, ls, le
			locs = append(locs, EvidenceLocation{
				Selector:  rule.Selector,
				ByteStart: &s1,
				ByteEnd:   &e1,
				LineStart: &ls1,
				LineEnd:   &le1,
			})
		}
	})
	if len(locs) > 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         rule.Key,
			RuleID:      rule.ID,
			Severity:    rule.Severity,
			Description: "css selector match",
			Locations:   condLocations(opts.RequestLocations, locs),
		})
		res.MatchedRules = append(res.MatchedRules, rule)
		res.Score += effectiveWeight(cfg, rule.ID, rule.Weight)
	}
}

// goqueryDocumentFromBytes parses HTML bytes into a goquery document.
func goqueryDocumentFromBytes(b []byte) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(bytes.NewReader(b))
}

// goqueryOuterHTML serializes the selection's first node outer HTML.
func goqueryOuterHTML(s *goquery.Selection) string {
	html, err := goquery.OuterHtml(s)
	if err != nil {
		return ""
	}
	return html
}

// indexOf finds the first index of sub in buf, returning -1 if not found.
func indexOf(buf, sub []byte) int {
	if len(sub) == 0 {
		return -1
	}
	return bytes.Index(buf, sub)
}
