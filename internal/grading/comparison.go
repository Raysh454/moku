package grading

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

// ComparisonTable renders a benchmark as an aligned matrix — one row per probe,
// one column per backend, each cell the outcome — followed by a per-backend
// summary of outcome counts and median latency.
func ComparisonTable(report BenchmarkReport) string {
	if len(report.Cards) == 0 {
		return "(no backends benchmarked)\n"
	}

	var b strings.Builder

	matrix := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	header := []string{"PROBE"}
	for _, card := range report.Cards {
		header = append(header, card.Backend)
	}
	fmt.Fprintln(matrix, strings.Join(header, "\t"))
	for index, base := range report.Cards[0].Results {
		row := []string{base.Probe.Name}
		for _, card := range report.Cards {
			row = append(row, string(outcomeFor(card, base.Probe.Name, index)))
		}
		fmt.Fprintln(matrix, strings.Join(row, "\t"))
	}
	_ = matrix.Flush()

	b.WriteString("\nsummary (ok/challenged/blocked/error, median p50):\n")
	summary := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, card := range report.Cards {
		counts := OutcomeCounts(card)
		fmt.Fprintf(summary, "%s\t%d/%d/%d/%d\tp50=%s\n",
			card.Backend,
			counts[OutcomeOK], counts[OutcomeChallenged], counts[OutcomeBlocked], counts[OutcomeError],
			medianP50(card))
	}
	_ = summary.Flush()

	return b.String()
}

// outcomeFor looks up a probe's outcome in a card, preferring the aligned index
// and falling back to a name match if the panels ever diverge in order.
func outcomeFor(card Scorecard, probeName string, index int) Outcome {
	if index < len(card.Results) && card.Results[index].Probe.Name == probeName {
		return card.Results[index].Outcome
	}
	for _, result := range card.Results {
		if result.Probe.Name == probeName {
			return result.Outcome
		}
	}
	return "-"
}

// medianP50 summarizes a backend's speed as the median of its per-probe p50s.
func medianP50(card Scorecard) time.Duration {
	p50s := make([]time.Duration, 0, len(card.Results))
	for _, result := range card.Results {
		p50s = append(p50s, result.Latency.P50)
	}
	return ComputeLatency(p50s).P50
}
