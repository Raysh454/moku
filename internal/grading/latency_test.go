package grading_test

import (
	"testing"
	"time"

	"github.com/raysh454/moku/internal/grading"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestComputeLatency_EmptySamples_ReturnsZeroStats(t *testing.T) {
	t.Parallel()

	stats := grading.ComputeLatency(nil)

	if stats.Samples != 0 {
		t.Errorf("expected 0 samples, got %d", stats.Samples)
	}
	if stats.Min != 0 || stats.Max != 0 || stats.P50 != 0 || stats.P95 != 0 {
		t.Errorf("expected all-zero stats for empty input, got %+v", stats)
	}
}

func TestComputeLatency_SingleSample_AllStatsEqualThatSample(t *testing.T) {
	t.Parallel()

	stats := grading.ComputeLatency([]time.Duration{ms(42)})

	if stats.Samples != 1 {
		t.Fatalf("expected 1 sample, got %d", stats.Samples)
	}
	for name, got := range map[string]time.Duration{
		"Min": stats.Min, "Max": stats.Max, "P50": stats.P50, "P95": stats.P95,
	} {
		if got != ms(42) {
			t.Errorf("expected %s=42ms, got %v", name, got)
		}
	}
}

func TestComputeLatency_UnsortedInput_SortsBeforeComputing(t *testing.T) {
	t.Parallel()

	// Nearest-rank percentile over 3 samples: P50 -> index ceil(.5*3)-1 = 1,
	// P95 -> index ceil(.95*3)-1 = 2.
	stats := grading.ComputeLatency([]time.Duration{ms(30), ms(10), ms(20)})

	if stats.Samples != 3 {
		t.Fatalf("expected 3 samples, got %d", stats.Samples)
	}
	if stats.Min != ms(10) {
		t.Errorf("expected Min=10ms, got %v", stats.Min)
	}
	if stats.Max != ms(30) {
		t.Errorf("expected Max=30ms, got %v", stats.Max)
	}
	if stats.P50 != ms(20) {
		t.Errorf("expected P50=20ms, got %v", stats.P50)
	}
	if stats.P95 != ms(30) {
		t.Errorf("expected P95=30ms, got %v", stats.P95)
	}
}

func TestComputeLatency_P95_PicksHighTailByNearestRank(t *testing.T) {
	t.Parallel()

	// 20 samples 1ms..20ms. P95 -> index ceil(.95*20)-1 = 18 -> 19ms.
	samples := make([]time.Duration, 0, 20)
	for i := 1; i <= 20; i++ {
		samples = append(samples, ms(i))
	}

	stats := grading.ComputeLatency(samples)

	if stats.P50 != ms(10) {
		t.Errorf("expected P50=10ms, got %v", stats.P50)
	}
	if stats.P95 != ms(19) {
		t.Errorf("expected P95=19ms, got %v", stats.P95)
	}
	if stats.Min != ms(1) || stats.Max != ms(20) {
		t.Errorf("expected Min=1ms Max=20ms, got Min=%v Max=%v", stats.Min, stats.Max)
	}
}
