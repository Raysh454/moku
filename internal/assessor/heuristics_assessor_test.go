package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

// TestNewHeuristicsAssessor_Construct verifies that NewHeuristicsAssessor returns a non-nil assessor
func TestNewHeuristicsAssessor_Construct(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, []assessor.Rule{}, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	if a == nil {
		t.Fatal("NewHeuristicsAssessor returned nil assessor")
	}
	defer a.Close()
}

// TestHeuristicsAssessor_ScoreHTML_Default verifies that ScoreHTML returns a neutral result
// with the expected evidence structure for the scaffold
func TestHeuristicsAssessor_ScoreHTML_Default(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, []assessor.Rule{}, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><h1>Test</h1></body></html>`)

	ctx := context.Background()
	snapshot := &models.Snapshot{ID: "test-snap", URL: "source", StatusCode: 200, Body: html}
	result, err := a.ScoreSnapshot(ctx, snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreHTML returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScoreHTML returned nil result")
	}

	if result.Score < 0.0 || result.Score > 1.0 {
		t.Errorf("Expected Score in [0.0,1.0], got %v", result.Score)
	}

	if len(result.Evidence) == 0 {
		t.Errorf("Expected at least 1 evidence item, got %d", len(result.Evidence))
	}

	if result.Confidence != cfg.DefaultConfidence {
		t.Errorf("Expected Confidence == %v, got %v", cfg.DefaultConfidence, result.Confidence)
	}

	if result.Version != cfg.ScoringVersion {
		t.Errorf("Expected Version == %q, got %q", cfg.ScoringVersion, result.Version)
	}
}

// TestHeuristicsAssessor_Close verifies that Close does not panic
func TestHeuristicsAssessor_Close(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, []assessor.Rule{}, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}

	err = a.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
