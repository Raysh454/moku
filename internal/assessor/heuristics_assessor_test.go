package assessor_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/webclient"
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

	// Small HTML fixture
	html := []byte(`<html><body><h1>Test</h1></body></html>`)

	ctx := context.Background()
	snapshot := &models.Snapshot{ID: "test-snap", URL: "source", StatusCode: 200, Body: html}
	result, err := a.ScoreSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("ScoreHTML returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScoreHTML returned nil result")
	}

	// Assert score is within expected bounds
	if result.Score < 0.0 || result.Score > 1.0 {
		t.Errorf("Expected Score in [0.0,1.0], got %v", result.Score)
	}

	// Assert we have at least one evidence item
	if len(result.Evidence) == 0 {
		t.Errorf("Expected at least 1 evidence item, got %d", len(result.Evidence))
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

func TestHeuristicsAssessor_ScoreResponse_ErrorsOnNilOrEmpty(t *testing.T) {
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	aRaw, err := assessor.NewHeuristicsAssessor(cfg, []assessor.Rule{}, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	defer aRaw.Close()

	a := aRaw.(*assessor.HeuristicsAssessor)

	if _, err := a.ScoreResponse(context.Background(), nil); err == nil {
		t.Errorf("expected error for nil response, got nil")
	}

	emptyResp := &webclient.Response{Body: nil}
	if _, err := a.ScoreResponse(context.Background(), emptyResp); err == nil {
		t.Errorf("expected error for empty body, got nil")
	}
}

func TestHeuristicsAssessor_ScoreResponse_Success(t *testing.T) {
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
	}
	logger := logging.NewStdoutLogger("assessor-test")

	aRaw, err := assessor.NewHeuristicsAssessor(cfg, []assessor.Rule{}, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	defer aRaw.Close()

	a := aRaw.(*assessor.HeuristicsAssessor)

	hdrs := http.Header{}
	hdrs.Set("Content-Type", "text/html; charset=utf-8")

	resp := &webclient.Response{
		Request: &webclient.Request{URL: "https://example.com"},
		Headers: hdrs,
		Body:    []byte("<html><body><h1>Test</h1></body></html>"),
		// Minimal status & timestamp
		StatusCode: 200,
		FetchedAt:  time.Now(),
	}

	res, err := a.ScoreResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ScoreResponse returned error: %v", err)
	}
	if res == nil {
		t.Fatal("ScoreResponse returned nil result")
	}
	if res.Score < 0.0 || res.Score > 1.0 {
		t.Errorf("ScoreResponse produced out-of-bounds score: %v", res.Score)
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
