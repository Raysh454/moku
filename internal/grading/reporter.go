package grading

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
)

// JSONReport renders a Scorecard as indented JSON suitable for archiving and
// diffing across runs.
func JSONReport(card Scorecard) ([]byte, error) {
	return json.MarshalIndent(card, "", "  ")
}

// TextReport renders a Scorecard as an aligned, human-readable table.
func TextReport(card Scorecard) string {
	var b strings.Builder
	fmt.Fprintf(&b, "backend: %s\n", card.Backend)

	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROBE\tOUTCOME\tP50\tP95\tSAMPLES\tEVIDENCE")
	for _, r := range card.Results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
			r.Probe.Name,
			r.Outcome,
			r.Latency.P50,
			r.Latency.P95,
			r.Latency.Samples,
			evidenceSummary(r),
		)
	}
	_ = w.Flush()
	return b.String()
}

// evidenceSummary condenses a probe's triggered signals (or error) into one cell.
func evidenceSummary(r ProbeResult) string {
	if r.Error != "" {
		return r.Error
	}
	names := make([]string, 0, len(r.Triggered))
	for _, sr := range r.Triggered {
		names = append(names, sr.Name)
	}
	return strings.Join(names, ",")
}
