package grading_test

import (
	"context"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/webclient"
)

func benchProbes() []grading.Probe {
	return []grading.Probe{{Name: "p", URL: "https://p"}}
}

func TestRunBenchmark_GradesEachNamedBackend(t *testing.T) {
	t.Parallel()

	clientA := &fakeWebClient{resp: map[string]*webclient.Response{"https://p": {StatusCode: 200, Body: []byte("ok")}}}
	clientB := &fakeWebClient{resp: map[string]*webclient.Response{"https://p": {StatusCode: 403, Body: []byte("no")}}}

	report := grading.RunBenchmark(
		context.Background(),
		[]grading.NamedClient{{Name: "A", Client: clientA}, {Name: "B", Client: clientB}},
		benchProbes(),
		grading.GraderConfig{Classifier: grading.DefaultClassifier(), Repeats: 1},
		&noopLogger{},
	)

	if len(report.Cards) != 2 {
		t.Fatalf("expected 2 scorecards, got %d", len(report.Cards))
	}
	if report.Cards[0].Backend != "A" || report.Cards[1].Backend != "B" {
		t.Errorf("expected backend labels A,B, got %q,%q", report.Cards[0].Backend, report.Cards[1].Backend)
	}
	if report.Cards[0].Results[0].Outcome != grading.OutcomeOK {
		t.Errorf("expected backend A OK, got %q", report.Cards[0].Results[0].Outcome)
	}
	if report.Cards[1].Results[0].Outcome != grading.OutcomeBlocked {
		t.Errorf("expected backend B blocked, got %q", report.Cards[1].Results[0].Outcome)
	}
}

func TestOutcomeCounts_TalliesOutcomes(t *testing.T) {
	t.Parallel()
	card := grading.Scorecard{Results: []grading.ProbeResult{
		{Outcome: grading.OutcomeOK},
		{Outcome: grading.OutcomeOK},
		{Outcome: grading.OutcomeBlocked},
	}}

	counts := grading.OutcomeCounts(card)
	if counts[grading.OutcomeOK] != 2 || counts[grading.OutcomeBlocked] != 1 {
		t.Errorf("unexpected counts: %v", counts)
	}
}

func TestComparisonTable_ContainsBackendsProbesAndOutcomes(t *testing.T) {
	t.Parallel()

	clientA := &fakeWebClient{resp: map[string]*webclient.Response{"https://p": {StatusCode: 200}}}
	clientB := &fakeWebClient{resp: map[string]*webclient.Response{"https://p": {StatusCode: 403}}}
	report := grading.RunBenchmark(
		context.Background(),
		[]grading.NamedClient{{Name: "alpha", Client: clientA}, {Name: "beta", Client: clientB}},
		benchProbes(),
		grading.GraderConfig{Classifier: grading.DefaultClassifier(), Repeats: 1},
		&noopLogger{},
	)

	table := grading.ComparisonTable(report)

	for _, want := range []string{"alpha", "beta", "p", string(grading.OutcomeOK), string(grading.OutcomeBlocked)} {
		if !strings.Contains(table, want) {
			t.Errorf("expected comparison table to contain %q, got:\n%s", want, table)
		}
	}
}

func TestComparisonTable_EmptyReport_IsSafe(t *testing.T) {
	t.Parallel()
	if got := grading.ComparisonTable(grading.BenchmarkReport{}); got == "" {
		t.Error("expected a non-empty placeholder for an empty report")
	}
}
