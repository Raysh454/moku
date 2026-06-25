package grading

import (
	"context"
	"time"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// GraderConfig configures a Grader. The zero value is usable: Repeats defaults
// to 1, Now defaults to time.Now, and a nil Classifier grades every non-nil
// response as OutcomeOK.
type GraderConfig struct {
	Classifier *Classifier
	// Repeats is the number of samples taken per probe (<=0 means 1).
	Repeats int
	// Now is an injectable clock, supplied by tests for deterministic latency.
	Now func() time.Time
	// Backend labels the scorecard with the backend under test.
	Backend string
	// PerRequestTimeout, when >0, caps each individual fetch so one hung target
	// cannot stall the whole panel.
	PerRequestTimeout time.Duration
}

// Grader fetches each probe through a WebClient, times and classifies every
// sample, and aggregates the results into a Scorecard. It owns orchestration
// only; detection lives in the Classifier and timing math in ComputeLatency.
type Grader struct {
	client        webclient.WebClient
	classifier    *Classifier
	repeats       int
	now           func() time.Time
	backend       string
	perReqTimeout time.Duration
	logger        logging.Logger
}

// NewGrader wires a Grader around a WebClient backend.
func NewGrader(client webclient.WebClient, cfg GraderConfig, logger logging.Logger) *Grader {
	repeats := cfg.Repeats
	if repeats <= 0 {
		repeats = 1
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	classifier := cfg.Classifier
	if classifier == nil {
		classifier = NewClassifier()
	}
	return &Grader{
		client:        client,
		classifier:    classifier,
		repeats:       repeats,
		now:           now,
		backend:       cfg.Backend,
		perReqTimeout: cfg.PerRequestTimeout,
		logger:        logger,
	}
}

// Grade runs the whole panel and returns the Scorecard.
func (g *Grader) Grade(ctx context.Context, probes []Probe) Scorecard {
	card := Scorecard{Backend: g.backend, Results: make([]ProbeResult, 0, len(probes))}
	for _, p := range probes {
		card.Results = append(card.Results, g.gradeProbe(ctx, p))
	}
	return card
}

// gradeProbe samples one probe g.repeats times and aggregates the result.
func (g *Grader) gradeProbe(ctx context.Context, probe Probe) ProbeResult {
	samples := make([]time.Duration, 0, g.repeats)
	result := ProbeResult{Probe: probe, Outcome: OutcomeOK, Outcomes: make(map[Outcome]int)}

	for range g.repeats {
		outcome, triggered, fetchErr := g.sample(ctx, probe.URL, &samples)
		result.Outcomes[outcome]++
		if fetchErr != "" {
			result.Error = fetchErr
		}
		if outcome.severity() > result.Outcome.severity() {
			result.Outcome = outcome
			result.Triggered = triggered
		}
	}

	result.Latency = ComputeLatency(samples)
	return result
}

// sample performs one timed fetch and classifies it, appending the measured
// latency to samples. A transport failure yields OutcomeError plus its message.
func (g *Grader) sample(ctx context.Context, url string, samples *[]time.Duration) (Outcome, []SignalResult, string) {
	if g.perReqTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.perReqTimeout)
		defer cancel()
	}

	start := g.now()
	resp, err := g.client.Get(ctx, url)
	*samples = append(*samples, g.now().Sub(start))

	if err != nil {
		g.logger.Debug("grading fetch failed",
			logging.Field{Key: "url", Value: url},
			logging.Field{Key: "error", Value: err.Error()})
		return OutcomeError, nil, err.Error()
	}
	cr := g.classifier.Classify(resp)
	return cr.Outcome, cr.Triggered, ""
}
