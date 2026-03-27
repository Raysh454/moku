package assessor_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

func TestDefaultRules_should_return_non_empty_well_formed_rules(t *testing.T) {
	t.Parallel()
	rules := assessor.DefaultRules()

	if len(rules) == 0 {
		t.Fatal("expected DefaultRules to return at least one rule")
	}

	for _, r := range rules {
		if r.ID == "" {
			t.Error("rule has empty ID")
		}
		if r.Key == "" {
			t.Errorf("rule %s has empty Key", r.ID)
		}
		if r.Severity == "" {
			t.Errorf("rule %s has empty Severity", r.ID)
		}
		if r.Weight <= 0 {
			t.Errorf("rule %s has non-positive Weight %v", r.ID, r.Weight)
		}
		if r.Selector == "" && r.Regex == "" {
			t.Errorf("rule %s has neither Selector nor Regex", r.ID)
		}
	}
}

func TestDefaultRules_should_have_unique_IDs(t *testing.T) {
	t.Parallel()
	rules := assessor.DefaultRules()

	seen := make(map[string]bool)
	for _, r := range rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule ID: %s", r.ID)
		}
		seen[r.ID] = true
	}
}

func TestDefaultRules_should_have_compilable_regex_patterns(t *testing.T) {
	t.Parallel()
	rules := assessor.DefaultRules()

	for _, r := range rules {
		if r.Regex == "" {
			continue
		}
		if _, err := regexp.Compile(r.Regex); err != nil {
			t.Errorf("rule %s has invalid regex %q: %v", r.ID, r.Regex, err)
		}
	}
}

func TestDefaultRules_InlineEventHandler_should_match_onclick(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("default-rules-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, assessor.DefaultRules(), logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><div onclick="alert(1)">click me</div></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-dr1", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	if _, ok := res.ContribByRule["dom:inline-event-handler"]; !ok {
		t.Error("expected dom:inline-event-handler rule to match onclick attribute")
	}
}

func TestDefaultRules_HardcodedSecret_should_match_api_key(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("default-rules-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, assessor.DefaultRules(), logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><script>var api_key = "sk-1234abcd";</script></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-dr2", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	if _, ok := res.ContribByRule["dom:hardcoded-secret"]; !ok {
		t.Error("expected dom:hardcoded-secret rule to match api_key assignment")
	}
}

func TestDefaultRules_JavascriptHref_should_match_javascript_link(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("default-rules-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, assessor.DefaultRules(), logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><a href="javascript:void(0)">link</a></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-dr3", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	if _, ok := res.ContribByRule["dom:javascript-href"]; !ok {
		t.Error("expected dom:javascript-href rule to match javascript: link")
	}
}

func TestDefaultRules_CleanHTML_should_not_trigger_dom_rules(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("default-rules-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, assessor.DefaultRules(), logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><head><title>Clean</title></head><body><p>Hello world</p></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-dr4", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	domPrefixRules := 0
	for ruleID := range res.ContribByRule {
		if len(ruleID) > 4 && ruleID[:4] == "dom:" {
			domPrefixRules++
		}
	}
	if domPrefixRules != 0 {
		t.Errorf("expected no dom: rules to match clean HTML, but %d matched: %v", domPrefixRules, res.ContribByRule)
	}
}
