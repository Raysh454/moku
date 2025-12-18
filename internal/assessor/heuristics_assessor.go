package assessor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
		return nil, errors.New("assessor: nil config; please pass a valid Config")
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
// Efficiently populates EvidenceLocation by using byte and line offsets.
func (h *HeuristicsAssessor) ScoreHTML(ctx context.Context, html []byte, source string) (*ScoreResult, error) {
	fmt.Println("HeuristicsAssessor: Scoring HTML from source:", source)
	fmt.Println("HTML bytes:", string(html))
	if h == nil || h.logger == nil {
		return nil, errors.New("heuristics-assessor: nil instance or logger")
	}

	doc, err := goqueryDocumentFromBytes(html)
	if err != nil {
		h.logger.Warn("couldn't convert html to goqueryDoc, skipping CSS only rules.", logging.Field{Key: "err", Value: err}, logging.Field{Key: "html", Value: html})
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
			appendRegexEvidence(html, byteToLine, r, res, h.logger, h.cfg.ScoreOpts)
		}
		if r.Selector != "" && doc != nil {
			appendCSSEvidence(doc, html, byteToLine, r, res, h.logger, h.cfg.ScoreOpts)
		}
	}

	// If no evidence matched, provide a scaffold default item for downstream expectations
	if len(res.Evidence) == 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         "no-evidence",
			Severity:    "low",
			Description: "no matching rules found",
			RuleID:      "scaffold:no_rules_matched",
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
func (h *HeuristicsAssessor) ScoreResponse(ctx context.Context, resp *webclient.Response) (*ScoreResult, error) {

	source := ""
	if resp != nil && resp.Request != nil {
		source = resp.Request.URL
	}
	if resp == nil || len(resp.Body) == 0 {
		return nil, errors.New("heuristics-assessor: nil response or empty body")
	}
	return h.ScoreHTML(ctx, resp.Body, source)
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

// Helpers extracted for clarity and testability

func buildLineStarts(html []byte) []int {
	starts := make([]int, 0, 1024)
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

func appendRegexEvidence(html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, logger logging.Logger, opts ScoreOptions) {
	if rule.compiled == nil {
		logger.Warn("regex rule skipped due to nil compiled regex", logging.Field{Key: "rule_id", Value: rule.ID})
		return
	}

	locs := []EvidenceLocation{}
	if opts.RequestLocations {
		for _, m := range rule.compiled.FindAllIndex(html, -1) {
			start, end := m[0], m[1]
			ls, le := byteToLine(start), byteToLine(end)
			locs = append(locs, EvidenceLocation{
				ByteStart: &start,
				ByteEnd:   &end,
				LineStart: &ls,
				LineEnd:   &le,
			})
		}
	}
	if len(locs) > 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         rule.Key,
			RuleID:      rule.ID,
			Severity:    rule.Severity,
			Description: "regex match",
			Locations:   locs,
		})
		res.MatchedRules = append(res.MatchedRules, rule)
		res.Score += rule.Weight
	}
}

func appendCSSEvidence(doc *goquery.Document, html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, logger logging.Logger, opts ScoreOptions) {
	if rule.Selector == "" || doc == nil {
		logger.Warn("css selector rule skipped due to empty selector or nil document", logging.Field{Key: "rule_id", Value: rule.ID})
		return
	}
	selection := doc.Find(rule.Selector)
	if selection.Length() == 0 {
		return
	}
	locs := []EvidenceLocation{}
	if opts.RequestLocations {
		selection.Each(func(i int, s *goquery.Selection) {
			outer := goqueryOuterHTML(s)
			idx := indexOf(html, []byte(outer))
			if idx >= 0 {
				start := idx
				end := idx + len(outer)
				ls, le := byteToLine(start), byteToLine(end)
				locs = append(locs, EvidenceLocation{
					Selector:  rule.Selector,
					ByteStart: &start,
					ByteEnd:   &end,
					LineStart: &ls,
					LineEnd:   &le,
				})
			}
		})
	}
	if len(locs) > 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:         rule.Key,
			RuleID:      rule.ID,
			Severity:    rule.Severity,
			Description: "css selector match",
			Locations:   locs,
		})
		res.MatchedRules = append(res.MatchedRules, rule)
		res.Score += rule.Weight
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
