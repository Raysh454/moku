package grading

// ProbeResult is the graded outcome of one probe across all its samples.
type ProbeResult struct {
	Probe Probe `json:"probe"`
	// Outcome is the most severe outcome observed across the samples.
	Outcome Outcome `json:"outcome"`
	// Outcomes counts how many samples landed in each outcome.
	Outcomes map[Outcome]int `json:"outcomes"`
	// Latency aggregates wall-clock latency across the samples.
	Latency LatencyStats `json:"latency"`
	// Triggered is the evidence from the most severe sample, if any.
	Triggered []SignalResult `json:"triggered,omitempty"`
	// Error is the last fetch error observed, if any sample failed.
	Error string `json:"error,omitempty"`
}

// Scorecard is the full grading result for one backend over a panel of probes.
type Scorecard struct {
	Backend string        `json:"backend"`
	Results []ProbeResult `json:"results"`
}
