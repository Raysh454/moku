package grading_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/grading"
)

func sampleScorecard() grading.Scorecard {
	return grading.Scorecard{
		Backend: "nethttp",
		Results: []grading.ProbeResult{{
			Probe:    grading.Probe{Name: "sannysoft", URL: "https://bot.sannysoft.com/"},
			Outcome:  grading.OutcomeOK,
			Outcomes: map[grading.Outcome]int{grading.OutcomeOK: 2},
			Latency:  grading.ComputeLatency([]time.Duration{10 * time.Millisecond, 30 * time.Millisecond}),
		}},
	}
}

func TestJSONReport_RoundTripsBackendAndOutcome(t *testing.T) {
	t.Parallel()

	data, err := grading.JSONReport(sampleScorecard())
	if err != nil {
		t.Fatalf("JSONReport: %v", err)
	}

	var decoded grading.Scorecard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-decoding report: %v", err)
	}
	if decoded.Backend != "nethttp" {
		t.Errorf("expected backend 'nethttp', got %q", decoded.Backend)
	}
	if len(decoded.Results) != 1 || decoded.Results[0].Outcome != grading.OutcomeOK {
		t.Errorf("expected one OK result, got %+v", decoded.Results)
	}
}

func TestTextReport_ContainsProbeNameAndOutcome(t *testing.T) {
	t.Parallel()

	out := grading.TextReport(sampleScorecard())

	if !strings.Contains(out, "nethttp") {
		t.Errorf("expected backend label in text report, got:\n%s", out)
	}
	if !strings.Contains(out, "sannysoft") {
		t.Errorf("expected probe name in text report, got:\n%s", out)
	}
	if !strings.Contains(out, string(grading.OutcomeOK)) {
		t.Errorf("expected outcome in text report, got:\n%s", out)
	}
}
