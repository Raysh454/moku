package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

func TestScoreHTML_RegexAndSelector_MatchesLocationsAndNormalization(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")

	rules := []assessor.Rule{
		{ID: "r1", Key: "regex-key", Severity: "medium", Regex: "<h1>Test</h1>", Weight: 0.4},
		{ID: "s1", Key: "selector-key", Severity: "high", Selector: "p", Weight: 0.6},
	}
	cfg.ScoreOpts = assessor.ScoreOptions{RequestLocations: true}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte("<html>\n<body>\n<h1>Test</h1>\n<p>para</p>\n</body>\n</html>\n")
	ctx := context.Background()

	snapshot := &models.Snapshot{ID: "snap-1", URL: "source", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(ctx, snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}

	if len(res.Evidence) < 2 {
		t.Fatalf("expected at least 2 evidence items, got %d", len(res.Evidence))
	}
	foundRegex := false
	foundSelector := false
	for _, ev := range res.Evidence {
		if ev.Description == "regex match" && len(ev.Locations) > 0 {
			foundRegex = true
			if ev.Locations[0].LineStart == nil || ev.Locations[0].LineEnd == nil {
				t.Error("regex locations missing line data")
			}
		}
		if ev.Description == "css selector match" && len(ev.Locations) > 0 {
			foundSelector = true
			if ev.Locations[0].LineStart == nil || ev.Locations[0].LineEnd == nil {
				t.Error("selector locations missing line data")
			}
		}
	}
	if !foundRegex || !foundSelector {
		t.Errorf("expected both regex and selector evidence; regex=%v selector=%v", foundRegex, foundSelector)
	}

	if res.Score < 0 || res.Score > 1 {
		t.Errorf("score out of bounds: %v", res.Score)
	}
	if res.Normalized < 0 || res.Normalized > 100 {
		t.Errorf("normalized out of bounds: %v", res.Normalized)
	}
}

func TestScoreHTML_NoLocationsRequested_SuppressesLocationData(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")
	rules := []assessor.Rule{{ID: "r1", Key: "k", Severity: "low", Regex: "para", Weight: 1.0}}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte("<html>\n<body>\n<p>para</p>\n</body>\n</html>")
	snapshot := &models.Snapshot{ID: "snap-2", URL: "src", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	if len(res.Evidence) == 0 {
		t.Fatal("expected evidence")
	}
	for _, ev := range res.Evidence {
		if len(ev.Locations) != 0 {
			t.Errorf("expected no locations when not requested, got %d", len(ev.Locations))
		}
	}
}

func TestScoreHTML_RegexWeight_should_affect_contribution(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")

	rules := []assessor.Rule{
		{ID: "r1", Key: "regex-key", Severity: "medium", Regex: "secret_value", Weight: 0.4},
	}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte("<html><body>secret_value</body></html>")
	snapshot := &models.Snapshot{ID: "snap-contrib-r", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	contrib, ok := res.ContribByRule["r1"]
	if !ok {
		t.Fatal("expected ContribByRule to contain key 'r1'")
	}
	if contrib != 0.4 {
		t.Errorf("expected ContribByRule['r1'] == 0.4, got %v", contrib)
	}
	if res.Score <= 0 {
		t.Errorf("expected Score > 0 when rule matches, got %v", res.Score)
	}
}

func TestScoreHTML_CSSWeight_should_affect_contribution(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")

	rules := []assessor.Rule{
		{ID: "s1", Key: "selector-key", Severity: "high", Selector: "div.target", Weight: 0.6},
	}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><div class="target">content</div></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-contrib-s", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	contrib, ok := res.ContribByRule["s1"]
	if !ok {
		t.Fatal("expected ContribByRule to contain key 's1'")
	}
	if contrib != 0.6 {
		t.Errorf("expected ContribByRule['s1'] == 0.6, got %v", contrib)
	}
	if res.Score <= 0 {
		t.Errorf("expected Score > 0 when rule matches, got %v", res.Score)
	}
}

func TestScoreHTML_MultipleRules_should_add_up_contributions(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")

	rules := []assessor.Rule{
		{ID: "r1", Key: "regex-key", Severity: "medium", Regex: "keyword", Weight: 0.3},
		{ID: "s1", Key: "selector-key", Severity: "high", Selector: "span.flag", Weight: 0.2},
	}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body>keyword<span class="flag">x</span></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-contrib-m", URL: "http://example.com/page", StatusCode: 200, Body: html}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}

	rContrib, rOk := res.ContribByRule["r1"]
	sContrib, sOk := res.ContribByRule["s1"]
	if !rOk {
		t.Fatal("expected ContribByRule to contain key 'r1'")
	}
	if !sOk {
		t.Fatal("expected ContribByRule to contain key 's1'")
	}
	if rContrib != 0.3 {
		t.Errorf("expected ContribByRule['r1'] == 0.3, got %v", rContrib)
	}
	if sContrib != 0.2 {
		t.Errorf("expected ContribByRule['s1'] == 0.2, got %v", sContrib)
	}
	if res.Score <= 0 {
		t.Errorf("expected Score > 0 when rules match, got %v", res.Score)
	}
}

func TestScoreHTML_NoRules_AddsDefaultEvidence(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")
	a, err := assessor.NewHeuristicsAssessor(cfg, nil, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	snapshot := &models.Snapshot{ID: "snap-3", URL: "src", StatusCode: 200, Body: []byte("<html></html>")}
	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}
	if len(res.Evidence) == 0 {
		t.Fatal("expected default evidence item when no rules matched")
	}
}
