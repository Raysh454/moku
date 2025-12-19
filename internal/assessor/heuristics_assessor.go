package assessor

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
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

// Not sure whether to keep both domRuleBasedScoring and scoreUsingAttackSurface
// or just one. For now, keeping both for flexibility.
func (h *HeuristicsAssessor) domRuleBasedScoring(snapshot *models.Snapshot, res *ScoreResult) error {

	doc, err := goqueryDocumentFromBytes(snapshot.Body)
	if err != nil {
		h.logger.Warn("couldn't convert html to goqueryDoc, skipping CSS only rules.", logging.Field{Key: "err", Value: err})
	}

	// Build compact line-start offsets and mapping helper
	lineStarts := buildLineStarts(snapshot.Body)
	byteToLine := func(pos int) int { return byteToLineFromStarts(lineStarts, pos) }

	// Run rules
	if h.cfg.ScoreOpts.MaxCSSEvidenceSamples <= 0 {
		h.cfg.ScoreOpts.MaxCSSEvidenceSamples = 10 // Default max samples
	}
	if h.cfg.ScoreOpts.MaxRegexEvidenceSamples <= 0 {
		h.cfg.ScoreOpts.MaxRegexEvidenceSamples = 10 // Default max samples
	}
	if h.cfg.ScoreOpts.MaxRegexMatchValueLen <= 0 {
		h.cfg.ScoreOpts.MaxRegexMatchValueLen = 200 // Default max length
	}

	urlTools, err := utils.NewURLTools(snapshot.URL)
	if err != nil {
		h.logger.Warn("failed to parse snapshot URL for path extraction", logging.Field{Key: "err", Value: err}, logging.Field{Key: "url", Value: snapshot.URL})
	}
	filePath := urlTools.GetPath()

	for _, r := range h.rules {

		// Moving to attack surface primarily, but keeping regex and css for now with contrib == 0.0
		if r.Regex != "" && r.compiled != nil {
			appendRegexEvidence(snapshot.Body, byteToLine, r, res, filePath, snapshot.ID, h.logger, h.cfg.ScoreOpts)
		}
		if r.Selector != "" && doc != nil {
			appendCSSEvidence(doc, snapshot.Body, byteToLine, r, res, filePath, snapshot.ID, h.logger, h.cfg.ScoreOpts)
		}
	}

	// If no evidence matched, provide a scaffold default item for downstream expectations
	if len(res.Evidence) == 0 {
		res.Evidence = append(res.Evidence, EvidenceItem{
			Key:          "no-evidence",
			Severity:     "low",
			Description:  "no matching rules found",
			RuleID:       "scaffold:no_rules_matched",
			Value:        "",
			Contribution: 0.0,
		})
	}

	return nil
}

func (h *HeuristicsAssessor) scoreUsingAttackSurface(snapshot *models.Snapshot, res *ScoreResult) (*attacksurface.AttackSurface, error) {
	at, err := attacksurface.BuildAttackSurfaceFromHTML(snapshot.URL, snapshot.ID, snapshot.StatusCode, snapshot.Headers, snapshot.Body)
	if err != nil {
		h.logger.Warn("failed to build attack surface from html",
			logging.Field{Key: "err", Value: err},
			logging.Field{Key: "snapshot_id", Value: snapshot.ID},
			logging.Field{Key: "url", Value: snapshot.URL},
		)
		return nil, err
	}

	if at != nil {
		feats := attacksurface.ComputeFeatures(at)
		res.RawFeatures = feats

		for name, val := range feats {
			w, ok := attacksurface.FeatureWeights[name]
			if !ok || val == 0 {
				continue
			}
			contrib := w * val

			locs := []EvidenceLocation{}
			if h.cfg.ScoreOpts.RequestLocations {
				locs = buildFeatureLocations(name, at)
			}

			res.Evidence = append(res.Evidence, EvidenceItem{
				Key:          name,
				RuleID:       name,
				Severity:     attacksurface.SeverityForFeature(name),
				Description:  attacksurface.DescribeFeature(name),
				Value:        val,
				Contribution: contrib,
				Locations:    locs,
			})
			res.ContribByRule[name] += contrib
		}
	}

	return at, nil
}

func (h *HeuristicsAssessor) ScoreSnapshot(ctx context.Context, snapshot *models.Snapshot, versionID string) (*ScoreResult, error) {
	if h == nil || h.logger == nil {
		return nil, errors.New("heuristics-assessor: nil instance or logger")
	}

	// Prepare result scaffolding
	res := &ScoreResult{
		Score:         0.0,
		SnapshotID:    snapshot.ID,
		VersionID:     versionID,
		Normalized:    0,
		Confidence:    h.cfg.DefaultConfidence,
		Version:       h.cfg.ScoringVersion,
		Evidence:      []EvidenceItem{},
		MatchedRules:  []Rule{},
		RawFeatures:   map[string]float64{},
		ContribByRule: map[string]float64{},
		Meta:          map[string]any{"snapshot_id": snapshot.ID, "url": snapshot.URL},
		Timestamp:     time.Now().UTC(),
	}

	as, err := h.scoreUsingAttackSurface(snapshot, res)
	if err != nil {
		h.logger.Warn("error during attack surface scoring", logging.Field{Key: "err", Value: err}, logging.Field{Key: "snapshot_id", Value: snapshot.ID})
	}
	res.AttackSurface = as

	err = h.domRuleBasedScoring(snapshot, res)
	if err != nil {
		h.logger.Warn("error during DOM rule-based scoring", logging.Field{Key: "err", Value: err}, logging.Field{Key: "snapshot_id", Value: snapshot.ID})
	}

	// Compute score from contributions
	var totalContrib float64
	for _, contrib := range res.ContribByRule {
		totalContrib += contrib
	}
	res.Score = normalizeScore(totalContrib)
	res.Normalized = int(res.Score * 100.0)

	return res, nil
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

func appendRegexEvidence(html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, filePath, snapshotID string, logger logging.Logger, opts ScoreOptions) {
	if rule.compiled == nil {
		logger.Warn("regex rule skipped due to nil compiled regex", logging.Field{Key: "rule_id", Value: rule.ID})
		return
	}

	matches := []string{}
	locs := []EvidenceLocation{}
	matchCount := 0

	if opts.RequestLocations {
		for _, m := range rule.compiled.FindAllIndex(html, -1) {
			start, end := m[0], m[1]
			ls, le := byteToLine(start), byteToLine(end)
			locs = append(locs, EvidenceLocation{
				Type:         "regex",
				FilePath:     filePath,
				Selector:     rule.Selector,
				RegexPattern: rule.Regex,
				SnapshotID:   snapshotID,
				ByteStart:    &start,
				ByteEnd:      &end,
				LineStart:    &ls,
				LineEnd:      &le,
			})
			sample := string(html[start:end])
			if len(sample) > opts.MaxRegexMatchValueLen {
				sample = sample[:opts.MaxRegexMatchValueLen] + "..."
			}

			if len(matches) < opts.MaxRegexEvidenceSamples {
				matches = append(matches, sample)
			}
			matchCount++
		}
	} else {
		for _, m := range rule.compiled.FindAllIndex(html, -1) {
			start, end := m[0], m[1]
			sample := string(html[start:end])
			if len(sample) > opts.MaxRegexMatchValueLen {
				sample = sample[:opts.MaxRegexMatchValueLen] + "..."
			}
			if len(matches) < opts.MaxRegexEvidenceSamples {
				matches = append(matches, sample)
			}
			matchCount++
		}
	}

	if matchCount == 0 {
		return
	}

	contribution := 0.0
	res.Evidence = append(res.Evidence, EvidenceItem{
		Key:         rule.Key,
		RuleID:      rule.ID,
		Severity:    rule.Severity,
		Description: "regex match",
		Value: map[string]any{
			"pattern":     rule.Regex,
			"match_count": matchCount,
			"samples":     matches,
		},
		Locations:    locs,
		Contribution: contribution,
	})
	res.MatchedRules = append(res.MatchedRules, rule)
	res.ContribByRule[rule.ID] += contribution
}

func appendCSSEvidence(doc *goquery.Document, html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, filePath, snapshotID string, logger logging.Logger, opts ScoreOptions) {
	if rule.Selector == "" || doc == nil {
		logger.Warn("css selector rule skipped due to empty selector or nil document", logging.Field{Key: "rule_id", Value: rule.ID})
		return
	}
	selection := doc.Find(rule.Selector)
	if selection.Length() == 0 {
		return
	}

	samples := []map[string]any{}
	matchCount := 0

	selection.Each(func(i int, s *goquery.Selection) {
		matchCount++

		if len(samples) >= opts.MaxCSSEvidenceSamples {
			return
		}

		node := s.Get(0)
		if node == nil {
			return
		}

		attrs := map[string]any{}
		for _, a := range node.Attr {
			// whitelist later
			if a.Key == "src" || a.Key == "href" || a.Key == "type" {
				attrs[a.Key] = a.Val
			}
		}

		samples = append(samples, map[string]any{
			"tag":   node.Data,
			"attrs": attrs,
		})
	})

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
					Type:         "css",
					FilePath:     filePath,
					SnapshotID:   snapshotID,
					Selector:     rule.Selector,
					RegexPattern: rule.Regex,
					ByteStart:    &start,
					ByteEnd:      &end,
					LineStart:    &ls,
					LineEnd:      &le,
				})
			}
		})
	}

	if matchCount == 0 {
		return
	}

	contribution := 0.0
	res.Evidence = append(res.Evidence, EvidenceItem{
		Key:         rule.Key,
		RuleID:      rule.ID,
		Severity:    rule.Severity,
		Description: "css selector match",
		Value: map[string]any{
			"selector":    rule.Selector,
			"match_count": matchCount,
			"samples":     samples,
		},
		Locations:    locs,
		Contribution: contribution,
	})
	res.MatchedRules = append(res.MatchedRules, rule)
	res.ContribByRule[rule.ID] += contribution
}

func buildFeatureLocations(featureName string, as *attacksurface.AttackSurface) []EvidenceLocation {
	if as == nil {
		return nil
	}

	switch featureName {

	// ---------- Headers ----------

	case "csp_missing", "csp_unsafe_inline", "csp_unsafe_eval":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "content-security-policy",
			SnapshotID: as.SnapshotID,
		}}

	case "xfo_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "x-frame-options",
			SnapshotID: as.SnapshotID,
		}}

	case "xcto_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "x-content-type-options",
			SnapshotID: as.SnapshotID,
		}}

	case "hsts_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "strict-transport-security",
			SnapshotID: as.SnapshotID,
		}}

	case "referrer_policy_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "referrer-policy",
			SnapshotID: as.SnapshotID,
		}}

	// ---------- Cookies ----------

	case "has_session_cookie_no_httponly":
		for _, c := range as.Cookies {
			if strings.Contains(strings.ToLower(c.Name), "session") && !c.HttpOnly {
				return []EvidenceLocation{{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				}}
			}
		}
		return nil

	case "num_cookies_missing_httponly":
		var locs []EvidenceLocation
		for _, c := range as.Cookies {
			if !c.HttpOnly {
				locs = append(locs, EvidenceLocation{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				})
			}
		}
		return locs

	case "num_cookies_missing_secure":
		var locs []EvidenceLocation
		for _, c := range as.Cookies {
			if !c.Secure {
				locs = append(locs, EvidenceLocation{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				})
			}
		}
		return locs

	// ---------- Forms & inputs ----------

	case "has_file_upload", "num_file_inputs":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			for _, in := range f.Inputs {
				if strings.EqualFold(in.Type, "file") {
					fIdx := f.DOMIndex
					iIdx := in.DOMIndex
					locs = append(locs, EvidenceLocation{
						Type:           "input",
						SnapshotID:     as.SnapshotID,
						ParentDOMIndex: &fIdx, // form index
						DOMIndex:       &iIdx, // input index within that form
					})
				}
			}
		}
		return locs

	case "has_password_input", "num_password_inputs":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			for _, in := range f.Inputs {
				if strings.EqualFold(in.Type, "password") {
					fIdx := f.DOMIndex
					iIdx := in.DOMIndex
					locs = append(locs, EvidenceLocation{
						Type:           "input",
						SnapshotID:     as.SnapshotID,
						ParentDOMIndex: &fIdx,
						DOMIndex:       &iIdx,
					})
				}
			}
		}
		return locs

	case "has_admin_form", "has_auth_form", "has_upload_form":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			fIdx := f.DOMIndex
			locs = append(locs, EvidenceLocation{
				Type:       "form",
				SnapshotID: as.SnapshotID,
				DOMIndex:   &fIdx,
			})
		}
		return locs

	// ---------- Params ----------

	case "has_admin_param", "has_upload_param", "has_debug_param", "has_id_param", "num_suspicious_params":
		// For now: just mark that suspicious params exist on this snapshot.
		// If you want per-name, you can store that in Evidence.Value, not Location.
		return []EvidenceLocation{{
			Type:       "param",
			SnapshotID: as.SnapshotID,
		}}

	default:
		return nil
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

// normalizeScore converts a raw contribution sum to a normalized score [0.0..1.0].
// This is a simple linear normalization with a cap at 1.0.
// Future implementations could use sigmoid, logarithmic, or other scaling functions.
func normalizeScore(rawScore float64) float64 {
	if rawScore < 0 {
		return 0.0
	}
	if rawScore > 1.0 {
		return 1.0
	}
	return rawScore
}
