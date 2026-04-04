package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

func TestNewHeuristicsAssessor_Construct(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
		Saturation:        attacksurface.DefaultSaturationConfig(),
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

func TestHeuristicsAssessor_ScoreHTML_Default(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
		Saturation:        attacksurface.DefaultSaturationConfig(),
	}
	logger := logging.NewStdoutLogger("assessor-test")

	a, err := assessor.NewHeuristicsAssessor(cfg, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor returned error: %v", err)
	}
	defer a.Close()

	html := []byte(`<html><body><h1>Test</h1></body></html>`)

	ctx := context.Background()
	snapshot := &models.Snapshot{ID: "test-snap", URL: "source", StatusCode: 200, Body: html}
	result, err := a.ScoreSnapshot(ctx, snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreSnapshot returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScoreSnapshot returned nil result")
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

func TestHeuristicsAssessor_ScoreSnapshot_WithSecurityHeaders(t *testing.T) {
	t.Parallel()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.1.0",
		DefaultConfidence: 0.5,
		Saturation:        attacksurface.DefaultSaturationConfig(),
	}
	logger := logging.NewStdoutLogger("assessor-test")
	a, err := assessor.NewHeuristicsAssessor(cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a.Close()

	// Page with security headers should have higher hardening
	html := []byte(`<html><body><form action="/login" method="POST"><input type="password" name="pw"></form></body></html>`)
	headersSecure := map[string][]string{
		"content-security-policy":   {"default-src 'none'; script-src 'self'"},
		"strict-transport-security": {"max-age=31536000"},
		"x-frame-options":           {"DENY"},
		"x-content-type-options":    {"nosniff"},
	}

	secureSnap := &models.Snapshot{ID: "secure-snap", URL: "https://example.com/login", StatusCode: 200, Body: html, Headers: headersSecure}
	secureResult, err := a.ScoreSnapshot(context.Background(), secureSnap, "v-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	insecureSnap := &models.Snapshot{ID: "insecure-snap", URL: "https://example.com/login", StatusCode: 200, Body: html, Headers: map[string][]string{}}
	insecureResult, err := a.ScoreSnapshot(context.Background(), insecureSnap, "v-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secureResult.HardeningScore <= insecureResult.HardeningScore {
		t.Errorf("expected secure page hardening (%v) > insecure (%v)", secureResult.HardeningScore, insecureResult.HardeningScore)
	}

	if secureResult.Score >= insecureResult.Score {
		t.Errorf("expected secure page posture score (%v) < insecure (%v) due to hardening", secureResult.Score, insecureResult.Score)
	}
}

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
