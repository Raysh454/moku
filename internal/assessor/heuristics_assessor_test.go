package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/model"
)

// TestNewHeuristicsAssessor_Construct verifies that NewHeuristicsAssessor returns a non-nil assessor
func TestNewHeuristicsAssessor_Construct(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, logger)
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

	a, err := assessor.NewHeuristicsAssessor(cfg, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	defer a.Close()

	// Small HTML fixture
	html := []byte(`<html><body><h1>Test</h1></body></html>`)
	source := "test-fixture"

	ctx := context.Background()
	result, err := a.ScoreHTML(ctx, html, source, model.ScoreOptions{})
	if err != nil {
		t.Fatalf("ScoreHTML returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScoreHTML returned nil result")
	}

	// Assert neutral score (scaffold default)
	if result.Score != 0.0 {
		t.Errorf("Expected Score == 0.0, got %v", result.Score)
	}

	// Assert evidence contains one item with Key == "no-evidence"
	if len(result.Evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(result.Evidence))
	}
	if len(result.Evidence) > 0 && result.Evidence[0].Key != "no-evidence" {
		t.Errorf("Expected evidence Key == 'no-evidence', got %q", result.Evidence[0].Key)
	}

	// Assert confidence matches config
	if result.Confidence != cfg.DefaultConfidence {
		t.Errorf("Expected Confidence == %v, got %v", cfg.DefaultConfidence, result.Confidence)
	}

	// Assert version matches config
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

	a, err := assessor.NewHeuristicsAssessor(cfg, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}

	err = a.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
