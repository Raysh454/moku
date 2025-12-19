package score

import (
	"math"
	"sort"

	"github.com/raysh454/moku/internal/assessor"
)

// RuleDelta represents the change in a rule's contribution between two versions.
type RuleDelta struct {
	RuleID    string  `json:"rule_id"`
	Severity  string  `json:"severity,omitempty"`
	Base      float64 `json:"base"`
	Head      float64 `json:"head"`
	Delta     float64 `json:"delta"`
	RuleKey   string  `json:"rule_key,omitempty"`
	RuleLabel string  `json:"rule_label,omitempty"` // human-readable description
}

// ScoreDelta represents the complete delta between two score results.
type ScoreDelta struct {
	URL         string      `json:"url"`
	BaseVersion string      `json:"base_version"`
	HeadVersion string      `json:"head_version"`
	BaseScore   float64     `json:"base_score"`
	HeadScore   float64     `json:"head_score"`
	Delta       float64     `json:"delta"`
	RuleDeltas  []RuleDelta `json:"rule_deltas"`
}

// DiffScoreResults computes the delta between two ScoreResults.
// It returns a ScoreDelta with per-rule contributions and changes.
func DiffScoreResults(
	base, head *assessor.ScoreResult,
	url, baseVersionID, headVersionID string,
) *ScoreDelta {
	sd := &ScoreDelta{
		URL:         url,
		BaseVersion: baseVersionID,
		HeadVersion: headVersionID,
		BaseScore:   0.0,
		HeadScore:   0.0,
		Delta:       0.0,
		RuleDeltas:  []RuleDelta{},
	}

	if base != nil {
		sd.BaseScore = base.Score
	}
	if head != nil {
		sd.HeadScore = head.Score
	}
	sd.Delta = sd.HeadScore - sd.BaseScore

	// Collect union of all rule IDs
	seen := make(map[string]struct{})
	addRule := func(id string) { seen[id] = struct{}{} }

	if base != nil && base.ContribByRule != nil {
		for id := range base.ContribByRule {
			addRule(id)
		}
	}
	if head != nil && head.ContribByRule != nil {
		for id := range head.ContribByRule {
			addRule(id)
		}
	}

	// Build a map of rule metadata from matched rules
	ruleMeta := make(map[string]assessor.Rule)
	if base != nil {
		for _, r := range base.MatchedRules {
			ruleMeta[r.ID] = r
		}
	}
	if head != nil {
		for _, r := range head.MatchedRules {
			ruleMeta[r.ID] = r
		}
	}

	// Compute deltas for each rule
	for ruleID := range seen {
		var b, h float64
		if base != nil && base.ContribByRule != nil {
			b = base.ContribByRule[ruleID]
		}
		if head != nil && head.ContribByRule != nil {
			h = head.ContribByRule[ruleID]
		}
		d := h - b

		rdelta := RuleDelta{
			RuleID: ruleID,
			Base:   b,
			Head:   h,
			Delta:  d,
		}

		// Fill in metadata if available
		if rule, ok := ruleMeta[ruleID]; ok {
			rdelta.Severity = rule.Severity
			rdelta.RuleKey = rule.Key
			rdelta.RuleLabel = rule.ID // or a better label if available
		}

		sd.RuleDeltas = append(sd.RuleDeltas, rdelta)
	}

	// Sort by absolute delta descending (most changed rules first)
	sort.Slice(sd.RuleDeltas, func(i, j int) bool {
		return math.Abs(sd.RuleDeltas[i].Delta) > math.Abs(sd.RuleDeltas[j].Delta)
	})

	return sd
}
