package tracker_test

import (
	"context"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
)

func TestNewSQLiteTracker(t *testing.T) {
	logger := logging.NewStdoutLogger("Tracker-test")
	cfg := &assessor.Config{ScoringVersion: "v0.1.0", DefaultConfidence: 0.5, RuleWeights: map[string]float64{"r1": 0.4, "s1": 0.6}}

	rules := []assessor.Rule{
		{ID: "r1", Key: "regex-key", Severity: "medium", Regex: "<h1>Test</h1>", Weight: 0.4},
		{ID: "s1", Key: "selector-key", Severity: "high", Selector: "p", Weight: 0.6},
	}
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("Failed to create HeuristicsAssessor: %v", err)
	}

	tr, err := tracker.NewSQLiteTracker(logger, a, &tracker.Config{StoragePath: "/tmp/moku"})
	if err != nil {
		t.Fatalf("Failed to create SQLiteTracker: %v", err)
	}
	defer tr.Close()

	s := tracker.Snapshot{
		ID:         "0",
		StatusCode: 200,
		URL:        "https://example.com/",
		Body:       []byte("<h1>Initial</h1>"),
		Headers:    map[string][]string{"IHeader": {"Init"}},
		CreatedAt:  time.Now(),
	}

	cr, err := tr.Commit(context.Background(), &s, "Initial Commit", "Author")
	if err != nil {
		t.Fatalf("Failed to commit snapshot: %v", err)
	}
	_ = tr.ScoreAndAttributeVersion(context.Background(), cr, &assessor.ScoreOptions{})

	s1 := tracker.Snapshot{
		ID:         "1",
		StatusCode: 200,
		URL:        "https://example.com/first",
		Body:       []byte("<h1>First</h1>"),
		Headers:    map[string][]string{"FHeader": {"A"}},
		CreatedAt:  time.Now(),
	}

	s2 := tracker.Snapshot{
		ID:         "2",
		StatusCode: 200,
		URL:        "https://example.com/second",
		Body:       []byte("<h1>Second</h1>"),
		Headers:    map[string][]string{"SHeader": {"B"}},
		CreatedAt:  time.Now(),
	}

	snapshots := []*tracker.Snapshot{&s1, &s2}

	cr, err = tr.CommitBatch(context.Background(), snapshots, "First Commit", "Author")
	if err != nil {
		t.Fatalf("Failed to commit batch snapshots: %v", err)
	}
	_ = tr.ScoreAndAttributeVersion(context.Background(), cr, &assessor.ScoreOptions{})

	s1 = tracker.Snapshot{
		ID:         "1",
		StatusCode: 200,
		URL:        "https://example.com/first",
		Body:       []byte("<h1>First Changed</h1>"),
		Headers:    map[string][]string{"FHeader": {"AA"}},
		CreatedAt:  time.Now(),
	}

	s2 = tracker.Snapshot{
		ID:         "2",
		StatusCode: 200,
		URL:        "https://example.com/second",
		Body:       []byte("<h1>Second Changed</h1>"),
		Headers:    map[string][]string{"SHeader": {"BB"}},
		CreatedAt:  time.Now(),
	}

	snapshots = []*tracker.Snapshot{&s1, &s2}

	cr, err = tr.CommitBatch(context.Background(), snapshots, "Second Commit", "Author")
	if err != nil {
		t.Fatalf("Failed to commit batch snapshots: %v", err)
	}
	_ = tr.ScoreAndAttributeVersion(context.Background(), cr, &assessor.ScoreOptions{})

	s = tracker.Snapshot{
		ID:         "2",
		StatusCode: 200,
		URL:        "https://example.com/second",
		Body:       []byte("<h1>Second</h1>"),
		Headers:    map[string][]string{"SHeader": {"BB"}},
		CreatedAt:  time.Now(),
	}

	cr, err = tr.Commit(context.Background(), &s, "Third Commit", "Author")
	if err != nil {
		t.Fatalf("Failed to commit snapshot: %v", err)
	}
	_ = tr.ScoreAndAttributeVersion(context.Background(), cr, &assessor.ScoreOptions{})

}
