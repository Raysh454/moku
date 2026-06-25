package grading_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// ─── test doubles ───────────────────────────────────────────────────────

type noopLogger struct{}

func (n *noopLogger) Debug(string, ...logging.Field)       {}
func (n *noopLogger) Info(string, ...logging.Field)        {}
func (n *noopLogger) Warn(string, ...logging.Field)        {}
func (n *noopLogger) Error(string, ...logging.Field)       {}
func (n *noopLogger) With(...logging.Field) logging.Logger { return n }

// fakeWebClient returns scripted responses/errors per URL and counts calls.
type fakeWebClient struct {
	resp  map[string]*webclient.Response
	err   map[string]error
	calls int
}

func (f *fakeWebClient) Get(_ context.Context, url string) (*webclient.Response, error) {
	f.calls++
	if e, ok := f.err[url]; ok {
		return nil, e
	}
	return f.resp[url], nil
}

func (f *fakeWebClient) Do(ctx context.Context, req *webclient.Request) (*webclient.Response, error) {
	return f.Get(ctx, req.URL)
}

func (f *fakeWebClient) Close() error { return nil }

// fakeClock returns successive scripted instants; Grade calls it twice per
// sample (start, end), so elapsed equals the gap between paired entries.
type fakeClock struct {
	times []time.Time
	i     int
}

func (c *fakeClock) now() time.Time {
	t := c.times[c.i]
	c.i++
	return t
}

func at(msOffsets ...int) []time.Time {
	base := time.Unix(0, 0)
	out := make([]time.Time, len(msOffsets))
	for i, off := range msOffsets {
		out[i] = base.Add(time.Duration(off) * time.Millisecond)
	}
	return out
}

func defaultClassifier() *grading.Classifier {
	return grading.NewClassifier(
		grading.ClassifyRule{Signal: grading.NewStatusSignal("blocked-status", 403, 503), Outcome: grading.OutcomeBlocked},
		grading.ClassifyRule{Signal: grading.NewBodyMarkerSignal("cf-challenge", "just a moment"), Outcome: grading.OutcomeChallenged},
	)
}

// ─── tests ──────────────────────────────────────────────────────────────

func TestGrader_RunsRepeatsPerProbe(t *testing.T) {
	t.Parallel()

	client := &fakeWebClient{resp: map[string]*webclient.Response{
		"https://a": {StatusCode: 200, Body: []byte("ok")},
		"https://b": {StatusCode: 200, Body: []byte("ok")},
	}}
	clock := &fakeClock{times: at(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)}
	g := grading.NewGrader(client, grading.GraderConfig{
		Classifier: defaultClassifier(),
		Repeats:    3,
		Now:        clock.now,
		Backend:    "fake",
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "a", URL: "https://a"}, {Name: "b", URL: "https://b"}})

	if client.calls != 6 {
		t.Errorf("expected 2 probes * 3 repeats = 6 calls, got %d", client.calls)
	}
	if len(card.Results) != 2 {
		t.Fatalf("expected 2 probe results, got %d", len(card.Results))
	}
	if card.Backend != "fake" {
		t.Errorf("expected backend label 'fake', got %q", card.Backend)
	}
}

func TestGrader_CleanResponse_OK_WithAggregatedLatency(t *testing.T) {
	t.Parallel()

	client := &fakeWebClient{resp: map[string]*webclient.Response{
		"https://ok": {StatusCode: 200, Body: []byte("<h1>hi</h1>")},
	}}
	// Two samples: start/end pairs (0,10) and (20,50) -> 10ms and 30ms.
	clock := &fakeClock{times: at(0, 10, 20, 50)}
	g := grading.NewGrader(client, grading.GraderConfig{
		Classifier: defaultClassifier(), Repeats: 2, Now: clock.now,
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "ok", URL: "https://ok"}})

	res := card.Results[0]
	if res.Outcome != grading.OutcomeOK {
		t.Errorf("expected OK, got %q", res.Outcome)
	}
	if res.Latency.Samples != 2 {
		t.Errorf("expected 2 latency samples, got %d", res.Latency.Samples)
	}
	if res.Latency.Min != 10*time.Millisecond || res.Latency.Max != 30*time.Millisecond {
		t.Errorf("expected latency min=10ms max=30ms, got min=%v max=%v", res.Latency.Min, res.Latency.Max)
	}
}

func TestGrader_ChallengeResponse_IsChallenged(t *testing.T) {
	t.Parallel()

	client := &fakeWebClient{resp: map[string]*webclient.Response{
		"https://cf": {StatusCode: 200, Body: []byte("<title>Just a moment...</title>")},
	}}
	clock := &fakeClock{times: at(0, 5)}
	g := grading.NewGrader(client, grading.GraderConfig{
		Classifier: defaultClassifier(), Repeats: 1, Now: clock.now,
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "cf", URL: "https://cf"}})

	res := card.Results[0]
	if res.Outcome != grading.OutcomeChallenged {
		t.Errorf("expected Challenged, got %q", res.Outcome)
	}
	if len(res.Triggered) == 0 {
		t.Error("expected triggered signal evidence on a challenged probe")
	}
}

func TestGrader_FetchError_IsErrorOutcome(t *testing.T) {
	t.Parallel()

	client := &fakeWebClient{err: map[string]error{
		"https://down": errors.New("connection refused"),
	}}
	clock := &fakeClock{times: at(0, 1, 2, 3)}
	g := grading.NewGrader(client, grading.GraderConfig{
		Classifier: defaultClassifier(), Repeats: 2, Now: clock.now,
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "down", URL: "https://down"}})

	res := card.Results[0]
	if res.Outcome != grading.OutcomeError {
		t.Errorf("expected Error outcome, got %q", res.Outcome)
	}
	if res.Error == "" {
		t.Error("expected the fetch error to be recorded")
	}
	if res.Outcomes[grading.OutcomeError] != 2 {
		t.Errorf("expected 2 error samples counted, got %d", res.Outcomes[grading.OutcomeError])
	}
}

func TestGrader_RepeatsDefaultsToOne(t *testing.T) {
	t.Parallel()

	client := &fakeWebClient{resp: map[string]*webclient.Response{
		"https://x": {StatusCode: 200, Body: []byte("ok")},
	}}
	clock := &fakeClock{times: at(0, 1)}
	g := grading.NewGrader(client, grading.GraderConfig{
		Classifier: defaultClassifier(), Repeats: 0, Now: clock.now,
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "x", URL: "https://x"}})

	if client.calls != 1 {
		t.Errorf("expected Repeats<=0 to default to 1 call, got %d", client.calls)
	}
	if card.Results[0].Latency.Samples != 1 {
		t.Errorf("expected 1 latency sample, got %d", card.Results[0].Latency.Samples)
	}
}

// blockingClient blocks until the request context is cancelled, modelling a
// hung target.
type blockingClient struct{}

func (b *blockingClient) Get(ctx context.Context, _ string) (*webclient.Response, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (b *blockingClient) Do(ctx context.Context, req *webclient.Request) (*webclient.Response, error) {
	return b.Get(ctx, req.URL)
}

func (b *blockingClient) Close() error { return nil }

func TestGrader_PerRequestTimeout_RecordsErrorInsteadOfHanging(t *testing.T) {
	t.Parallel()

	g := grading.NewGrader(&blockingClient{}, grading.GraderConfig{
		Classifier:        defaultClassifier(),
		Repeats:           1,
		PerRequestTimeout: 20 * time.Millisecond,
	}, &noopLogger{})

	card := g.Grade(context.Background(), []grading.Probe{{Name: "slow", URL: "https://slow"}})

	if card.Results[0].Outcome != grading.OutcomeError {
		t.Errorf("expected per-request timeout to yield Error, got %q", card.Results[0].Outcome)
	}
}
