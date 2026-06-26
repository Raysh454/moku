package grading

import (
	"context"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// NamedClient pairs a backend with the label it appears under in a benchmark.
type NamedClient struct {
	Name   string
	Client webclient.WebClient
}

// BenchmarkReport holds one Scorecard per benchmarked backend, in input order.
type BenchmarkReport struct {
	Cards []Scorecard `json:"cards"`
}

// RunBenchmark grades every backend through the same panel with the same grading
// configuration, so their outcomes and latencies are directly comparable. The
// supplied cfg's Backend field is ignored; each card is labelled with its
// NamedClient.Name.
func RunBenchmark(ctx context.Context, clients []NamedClient, probes []Probe, cfg GraderConfig, logger logging.Logger) BenchmarkReport {
	report := BenchmarkReport{Cards: make([]Scorecard, 0, len(clients))}
	for _, named := range clients {
		graderCfg := cfg
		graderCfg.Backend = named.Name
		grader := NewGrader(named.Client, graderCfg, logger)
		report.Cards = append(report.Cards, grader.Grade(ctx, probes))
	}
	return report
}

// OutcomeCounts tallies how many probes landed in each outcome for a scorecard.
func OutcomeCounts(card Scorecard) map[Outcome]int {
	counts := make(map[Outcome]int)
	for _, result := range card.Results {
		counts[result.Outcome]++
	}
	return counts
}
