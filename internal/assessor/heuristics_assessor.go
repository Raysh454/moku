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
)

type HeuristicsAssessor struct {
	cfg    *Config
	logger logging.Logger
	rules  []Rule
}

const (
	defaultMaxCSSEvidenceSamples   = 10
	defaultMaxRegexEvidenceSamples = 10
	defaultMaxRegexMatchValueLen   = 200
)

func NewHeuristicsAssessor(cfg *Config, rules []Rule, logger logging.Logger) (Assessor, error) {
	if cfg == nil {
		return nil, errors.New("assessor: nil config; please pass a valid Config")
	}
	if logger == nil {
		return nil, errors.New("assessor: nil logger; please pass a valid logging.Logger")
	}

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

func (h *HeuristicsAssessor) domRuleBasedScoring(snapshot *models.Snapshot, res *ScoreResult) error {

	doc, err := goqueryDocumentFromBytes(snapshot.Body)
	if err != nil {
		h.logger.Warn("couldn't convert html to goqueryDoc, skipping CSS only rules.", logging.Field{Key: "err", Value: err})
	}

	lineStartOffsets := buildLineStarts(snapshot.Body)
	byteToLine := func(position int) int { return byteToLineFromStarts(lineStartOffsets, position) }

	h.ensureScoreOptionDefaults()

	urlTools, err := utils.NewURLTools(snapshot.URL)
	if err != nil {
		h.logger.Warn("failed to parse snapshot URL for path extraction", logging.Field{Key: "err", Value: err}, logging.Field{Key: "url", Value: snapshot.URL})
	}
	filePath := urlTools.GetPath()

	for _, r := range h.rules {
		if r.Regex != "" && r.compiled != nil {
			appendRegexEvidence(snapshot.Body, byteToLine, r, res, filePath, snapshot.ID, h.logger, h.cfg.ScoreOpts)
		}
		if r.Selector != "" && doc != nil {
			appendCSSEvidence(doc, snapshot.Body, byteToLine, r, res, filePath, snapshot.ID, h.logger, h.cfg.ScoreOpts)
		}
	}

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

	res := newScoreResult(snapshot, versionID, h.cfg)

	as, err := h.scoreUsingAttackSurface(snapshot, res)
	if err != nil {
		h.logger.Warn("error during attack surface scoring", logging.Field{Key: "err", Value: err}, logging.Field{Key: "snapshot_id", Value: snapshot.ID})
	}
	res.AttackSurface = as

	err = h.domRuleBasedScoring(snapshot, res)
	if err != nil {
		h.logger.Warn("error during DOM rule-based scoring", logging.Field{Key: "err", Value: err}, logging.Field{Key: "snapshot_id", Value: snapshot.ID})
	}

	h.computeScore(res)

	return res, nil
}

func (*HeuristicsAssessor) computeScore(res *ScoreResult) {
	var totalContrib float64
	for _, contrib := range res.ContribByRule {
		totalContrib += contrib
	}
	res.Score = normalizeScore(totalContrib)
	res.Normalized = int(res.Score * 100.0)
}

func (h *HeuristicsAssessor) Close() error {
	if h == nil || h.logger == nil {
		return nil
	}
	h.logger.Info("heuristics-assessor: closed")
	return nil
}

func (h *HeuristicsAssessor) ensureScoreOptionDefaults() {
	if h.cfg.ScoreOpts.MaxCSSEvidenceSamples <= 0 {
		h.cfg.ScoreOpts.MaxCSSEvidenceSamples = defaultMaxCSSEvidenceSamples
	}
	if h.cfg.ScoreOpts.MaxRegexEvidenceSamples <= 0 {
		h.cfg.ScoreOpts.MaxRegexEvidenceSamples = defaultMaxRegexEvidenceSamples
	}
	if h.cfg.ScoreOpts.MaxRegexMatchValueLen <= 0 {
		h.cfg.ScoreOpts.MaxRegexMatchValueLen = defaultMaxRegexMatchValueLen
	}
}

func newScoreResult(snapshot *models.Snapshot, versionID string, cfg *Config) *ScoreResult {
	return &ScoreResult{
		Score:         0.0,
		SnapshotID:    snapshot.ID,
		VersionID:     versionID,
		Normalized:    0,
		Confidence:    cfg.DefaultConfidence,
		Version:       cfg.ScoringVersion,
		Evidence:      []EvidenceItem{},
		MatchedRules:  []Rule{},
		RawFeatures:   map[string]float64{},
		ContribByRule: map[string]float64{},
		Meta:          map[string]any{"snapshot_id": snapshot.ID, "url": snapshot.URL},
		Timestamp:     time.Now().UTC(),
	}
}

func buildLineStarts(html []byte) []int {
	startIndexes := make([]int, 0, 1024)
	startIndexes = append(startIndexes, 0)
	for i, byteValue := range html {
		if byteValue == '\n' {
			startIndexes = append(startIndexes, i+1)
		}
	}
	return startIndexes
}

func byteToLineFromStarts(lineStartOffsets []int, position int) int {
	lowerBound, upperBound := 0, len(lineStartOffsets)-1
	for lowerBound <= upperBound {
		midpoint := (lowerBound + upperBound) / 2
		start := lineStartOffsets[midpoint]
		if start == position {
			return midpoint + 1
		}
		if start < position {
			lowerBound = midpoint + 1
		} else {
			upperBound = midpoint - 1
		}
	}
	return lowerBound
}

func appendRegexEvidence(html []byte, byteToLine func(int) int, rule Rule, res *ScoreResult, filePath, snapshotID string, logger logging.Logger, opts ScoreOptions) {
	if rule.compiled == nil {
		logger.Warn("regex rule skipped due to nil compiled regex", logging.Field{Key: "rule_id", Value: rule.ID})
		return
	}

	matches, locs, matchCount := collectRegexEvidenceSamples(html, byteToLine, rule, filePath, snapshotID, opts)

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

func collectRegexEvidenceSamples(html []byte, byteToLine func(int) int, rule Rule, filePath, snapshotID string, opts ScoreOptions) ([]string, []EvidenceLocation, int) {
	matches := []string{}
	locs := []EvidenceLocation{}
	matchCount := 0

	for _, m := range rule.compiled.FindAllIndex(html, -1) {
		start, end := m[0], m[1]

		if opts.RequestLocations {
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
		}

		sample := string(html[start:end])
		if len(sample) > opts.MaxRegexMatchValueLen {
			sample = sample[:opts.MaxRegexMatchValueLen] + "..."
		}
		if len(matches) < opts.MaxRegexEvidenceSamples {
			matches = append(matches, sample)
		}
		matchCount++
	}

	return matches, locs, matchCount
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
						ParentDOMIndex: &fIdx,
						DOMIndex:       &iIdx,
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

	case "has_admin_param", "has_upload_param", "has_debug_param", "has_id_param", "num_suspicious_params":
		return []EvidenceLocation{{
			Type:       "param",
			SnapshotID: as.SnapshotID,
		}}

	default:
		return nil
	}
}

func goqueryDocumentFromBytes(b []byte) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(bytes.NewReader(b))
}

func goqueryOuterHTML(s *goquery.Selection) string {
	html, err := goquery.OuterHtml(s)
	if err != nil {
		return ""
	}
	return html
}

func indexOf(buf, sub []byte) int {
	if len(sub) == 0 {
		return -1
	}
	return bytes.Index(buf, sub)
}

func normalizeScore(rawScore float64) float64 {
	if rawScore < 0 {
		return 0.0
	}
	if rawScore > 1.0 {
		return 1.0
	}
	return rawScore
}
