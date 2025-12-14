package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
)

func TestScoreHTML_RegexAndSelector_MatchesLocationsAndNormalization(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5, RuleWeights: map[string]float64{"r1": 0.4, "s1": 0.6}}
	logger := logging.NewStdoutLogger("assessor-extensive-test")

	rules := []assessor.Rule{
		{ID: "r1", Key: "regex-key", Severity: "medium", Regex: "<h1>Test</h1>", Weight: 0.4},
		{ID: "s1", Key: "selector-key", Severity: "high", Selector: "p", Weight: 0.6},
	}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	html := []byte("<html>\n<body>\n<h1>Test</h1>\n<p>para</p>\n</body>\n</html>\n")
	ctx := context.Background()

	res, err := a.ScoreHTML(ctx, html, "source", assessor.ScoreOptions{RequestLocations: true})
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}

	if len(res.Evidence) < 2 {
		t.Fatalf("expected at least 2 evidence items, got %d", len(res.Evidence))
	}
	// Verify locations are present and line numbers reasonable
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

	// Score normalization bounds
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
	res, err := a.ScoreHTML(context.Background(), html, "src", assessor.ScoreOptions{RequestLocations: false})
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

func TestScoreHTML_NoRules_AddsDefaultEvidence(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5}
	logger := logging.NewStdoutLogger("assessor-extensive-test")
	a, err := assessor.NewHeuristicsAssessor(cfg, nil, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	defer a.Close()

	res, err := a.ScoreHTML(context.Background(), []byte("<html></html>"), "src", assessor.ScoreOptions{})
	if err != nil {
		t.Fatalf("ScoreHTML error: %v", err)
	}
	if len(res.Evidence) == 0 {
		t.Fatal("expected default evidence item when no rules matched")
	}
}
