package score

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create minimal schema
	schema := `
		CREATE TABLE score_results (
			id TEXT PRIMARY KEY,
			version_id TEXT NOT NULL,
			score REAL NOT NULL,
			normalized INTEGER NOT NULL,
			confidence REAL NOT NULL,
			scoring_version TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			score_json TEXT,
			matched_rules TEXT,
			meta TEXT,
			raw_features TEXT
		);
		CREATE UNIQUE INDEX idx_score_results_version ON score_results(version_id);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func TestGetScoreResultForVersion_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	result, err := GetScoreResultForVersion(ctx, db, "nonexistent-version")

	if err != nil {
		t.Errorf("Expected no error for nonexistent version, got %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for nonexistent version, got %+v", result)
	}
}

func TestGetScoreResultForVersion_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Insert a test score result
	scoreRes := &assessor.ScoreResult{
		Score:         0.5,
		Normalized:    50,
		Confidence:    0.9,
		Version:       "v1.0.0",
		ContribByRule: map[string]float64{"rule1": 0.3, "rule2": 0.2},
		Evidence:      []assessor.EvidenceItem{},
		MatchedRules:  []assessor.Rule{},
		Meta:          map[string]any{"test": "data"},
		RawFeatures:   map[string]float64{},
	}

	scoreJSON, err := json.Marshal(scoreRes)
	if err != nil {
		t.Fatalf("Failed to marshal score result: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO score_results (id, version_id, score, normalized, confidence, scoring_version, created_at, score_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "score-id-1", "version-1", 0.5, 50, 0.9, "v1.0.0", 123456, string(scoreJSON))
	if err != nil {
		t.Fatalf("Failed to insert test score: %v", err)
	}

	// Retrieve the score result
	result, err := GetScoreResultForVersion(ctx, db, "version-1")
	if err != nil {
		t.Fatalf("Failed to get score result: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Score != 0.5 {
		t.Errorf("Expected score 0.5, got %f", result.Score)
	}
	if result.Normalized != 50 {
		t.Errorf("Expected normalized 50, got %d", result.Normalized)
	}
	if len(result.ContribByRule) != 2 {
		t.Errorf("Expected 2 contrib rules, got %d", len(result.ContribByRule))
	}
	if result.ContribByRule["rule1"] != 0.3 {
		t.Errorf("Expected rule1 contrib 0.3, got %f", result.ContribByRule["rule1"])
	}
}

func TestCompareVersionsForURL_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Insert two score results
	baseScore := &assessor.ScoreResult{
		Score:         0.2,
		Normalized:    20,
		ContribByRule: map[string]float64{"rule1": 0.2},
	}
	headScore := &assessor.ScoreResult{
		Score:         0.5,
		Normalized:    50,
		ContribByRule: map[string]float64{"rule1": 0.2, "rule2": 0.3},
	}

	baseJSON, _ := json.Marshal(baseScore)
	headJSON, _ := json.Marshal(headScore)

	_, err := db.ExecContext(ctx, `
		INSERT INTO score_results (id, version_id, score, normalized, confidence, scoring_version, created_at, score_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "score-base", "version-base", 0.2, 20, 0.9, "v1", 100, string(baseJSON))
	if err != nil {
		t.Fatalf("Failed to insert base score: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO score_results (id, version_id, score, normalized, confidence, scoring_version, created_at, score_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "score-head", "version-head", 0.5, 50, 0.9, "v1", 200, string(headJSON))
	if err != nil {
		t.Fatalf("Failed to insert head score: %v", err)
	}

	// Compare versions
	delta, err := CompareVersionsForURL(ctx, db, "version-base", "version-head", "https://example.com")
	if err != nil {
		t.Fatalf("Failed to compare versions: %v", err)
	}

	if delta == nil {
		t.Fatal("Expected non-nil delta")
	}

	if delta.BaseScore != 0.2 {
		t.Errorf("Expected base score 0.2, got %f", delta.BaseScore)
	}
	if delta.HeadScore != 0.5 {
		t.Errorf("Expected head score 0.5, got %f", delta.HeadScore)
	}

	expectedDelta := 0.3
	if delta.Delta < expectedDelta-0.0001 || delta.Delta > expectedDelta+0.0001 {
		t.Errorf("Expected delta around 0.3, got %f", delta.Delta)
	}

	if len(delta.RuleDeltas) != 2 {
		t.Errorf("Expected 2 rule deltas, got %d", len(delta.RuleDeltas))
	}
}

func TestCompareVersionsForURL_MissingVersions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Test with empty version IDs
	_, err := CompareVersionsForURL(ctx, db, "", "version-2", "https://example.com")
	if err == nil {
		t.Error("Expected error for empty base version ID")
	}

	_, err = CompareVersionsForURL(ctx, db, "version-1", "", "https://example.com")
	if err == nil {
		t.Error("Expected error for empty head version ID")
	}
}
