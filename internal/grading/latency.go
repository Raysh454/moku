// Package grading measures how a webclient.WebClient backend performs against a
// panel of detection/challenge probes: whether each fetch comes back clean,
// challenged, or blocked, and how fast it is. It exists so a backend can be
// graded empirically — and regression-tested in CI — on the
// fast / undetectable axes rather than by reputation.
package grading

import (
	"math"
	"slices"
	"time"
)

// LatencyStats summarizes the wall-clock latency of a set of fetch samples.
// The zero value (all fields 0) represents "no samples".
type LatencyStats struct {
	Samples int           `json:"samples"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	P50     time.Duration `json:"p50"`
	P95     time.Duration `json:"p95"`
}

// ComputeLatency aggregates raw latency samples into LatencyStats using the
// nearest-rank percentile method. The input slice is not mutated. An empty or
// nil input yields the zero LatencyStats.
func ComputeLatency(samples []time.Duration) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}

	sorted := slices.Clone(samples)
	slices.Sort(sorted)

	return LatencyStats{
		Samples: len(sorted),
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		P50:     nearestRank(sorted, 50),
		P95:     nearestRank(sorted, 95),
	}
}

// nearestRank returns the p-th percentile (0 < p <= 100) of an ascending-sorted,
// non-empty slice using the nearest-rank method: rank = ceil(p/100 * n).
func nearestRank(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	rank := int(math.Ceil((p / 100.0) * float64(n)))
	index := min(max(rank-1, 0), n-1)
	return sorted[index]
}
