package tracker

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

// dummyAssessor returns a preconfigured ScoreResult for tests.
type dummyAssessor struct {
	res *model.ScoreResult
}

func (d *dummyAssessor) ScoreHTML(ctx context.Context, html []byte, source string, opts model.ScoreOptions) (*model.ScoreResult, error) {
	// return a deep copy so tests can mutate without colliding
	b, _ := json.Marshal(d.res)
	var out model.ScoreResult
	_ = json.Unmarshal(b, &out)
	return &out, nil
}
func (d *dummyAssessor) ScoreResponse(ctx context.Context, resp *model.Response, opts model.ScoreOptions) (*model.ScoreResult, error) {
	return d.ScoreHTML(ctx, nil, "", model.ScoreOptions{})
}
func (d *dummyAssessor) ExtractEvidence(ctx context.Context, html []byte, opts model.ScoreOptions) ([]model.EvidenceItem, error) {
	if d.res == nil {
		return nil, nil
	}
	b, _ := json.Marshal(d.res.Evidence)
	var out []model.EvidenceItem
	_ = json.Unmarshal(b, &out)
	return out, nil
}
func (d *dummyAssessor) Close() error { return nil }

// openTestDB creates an in-memory sqlite DB and creates the minimal tables used by the tests.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		t.Fatalf("pragmas: %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS versions (
		id TEXT PRIMARY KEY,
		parent_id TEXT,
		snapshot_id TEXT,
		message TEXT,
		author TEXT,
		timestamp INTEGER
	);

	CREATE TABLE IF NOT EXISTS diffs (
		id TEXT PRIMARY KEY,
		base_version_id TEXT,
		head_version_id TEXT NOT NULL,
		diff_json TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS version_scores (
		id TEXT PRIMARY KEY,
		version_id TEXT NOT NULL,
		scoring_version TEXT NOT NULL,
		score REAL NOT NULL,
		normalized INTEGER,
		confidence REAL,
		score_json TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS version_evidence_locations (
	  id TEXT PRIMARY KEY,
	  version_id TEXT NOT NULL,
	  evidence_id TEXT NOT NULL,
	  evidence_index INTEGER NOT NULL,
	  location_index INTEGER NOT NULL,
	  evidence_key TEXT,
	  selector TEXT,
	  xpath TEXT,
	  node_id TEXT,
	  file_path TEXT,
	  byte_start INTEGER,
	  byte_end INTEGER,
	  line_start INTEGER,
	  line_end INTEGER,
	  loc_confidence REAL,
	  evidence_json TEXT NOT NULL,
	  location_json TEXT,
	  scoring_version TEXT,
	  created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS diff_attributions (
	  id TEXT PRIMARY KEY,
	  diff_id TEXT NOT NULL,
	  head_version_id TEXT NOT NULL,
	  version_evidence_location_id TEXT,
	  evidence_id TEXT NOT NULL,
	  evidence_location_index INTEGER,
	  chunk_index INTEGER NOT NULL,
	  evidence_key TEXT,
	  evidence_json TEXT,
	  location_json TEXT,
	  scoring_version TEXT NOT NULL,
	  contribution REAL NOT NULL,
	  contribution_pct REAL NOT NULL,
	  note TEXT,
	  created_at INTEGER NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func countRows(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var cnt int
	if err := db.QueryRow(q, args...).Scan(&cnt); err != nil {
		t.Fatalf("countRows query failed: %v", err)
	}
	return cnt
}

// Test initial page (no parent) persists version_scores and version_evidence_locations
func TestScoreAndAttributeVersion_InitialPage_PersistsEvidenceLocations(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	score := &model.ScoreResult{
		Score:      0.8,
		Normalized: 80,
		Confidence: 0.9,
		Version:    "v-test-1",
		Evidence: []model.EvidenceItem{
			{
				ID:       "ev-1",
				Key:      "login-form",
				RuleID:   "forms:login",
				Severity: "high",
				Locations: []model.EvidenceLocation{
					{Selector: "form#login", LineStart: intPtr(10), LineEnd: intPtr(12), Confidence: floatPtr(1.0)},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := interfaces.Logger(nil)

	versionID := uuid.New().String()

	// Call with no parent/diff
	if err := ScoreAndAttributeVersion(context.Background(), db, logger, assr, model.ScoreOptions{RequestLocations: true}, versionID, "", "", "", []byte(`<html><body><form id="login"></form></body></html>`)); err != nil {
		t.Fatalf("ScoreAndAttributeVersion failed: %v", err)
	}

	// version_scores row exists
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM version_scores WHERE version_id = ?`, versionID); cnt != 1 {
		t.Fatalf("expected 1 version_scores row, got %d", cnt)
	}

	// one version_evidence_locations row inserted
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM version_evidence_locations WHERE version_id = ?`, versionID); cnt != 1 {
		t.Fatalf("expected 1 evidence location row, got %d", cnt)
	}

	// no diff_attributions (no diff provided)
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM diff_attributions WHERE head_version_id = ?`, versionID); cnt != 0 {
		t.Fatalf("expected 0 diff_attributions for initial page, got %d", cnt)
	}
}

// Test that attribution with a diff maps locations to the expected chunk index and inserts diff_attributions
func TestScoreAndAttributeVersion_WithDiff_AttributesLocations(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Insert a parent version score (so delta logging path exists)
	parentVersionID := uuid.New().String()
	parentScoreID := uuid.New().String()
	parentScoreJSON := `{"score":0.2}`
	if _, err := db.Exec(`INSERT INTO version_scores (id, version_id, scoring_version, score, normalized, confidence, score_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		parentScoreID, parentVersionID, "v-base", 0.2, 20, 0.5, parentScoreJSON, time.Now().Unix()); err != nil {
		t.Fatalf("insert parent score: %v", err)
	}

	// diff JSON: chunk 1 contains the login form
	diffID := uuid.New().String()
	diffJSON := `{
		"body_diff": {
			"chunks": [
				{"type":"removed","content":"<div id=\"old\">Old</div>"},
				{"type":"added","content":"<form id=\"login\"><input name=\"username\"/></form>"},
				{"type":"added","content":"<style>.btn{color:red}</style>"}
			]
		}
	}`

	// assessor returns evidence with selector matching form#login
	score := &model.ScoreResult{
		Score:      0.7,
		Normalized: 70,
		Confidence: 0.9,
		Version:    "v-test-2",
		Evidence: []model.EvidenceItem{
			{
				ID:       "ev-2",
				Key:      "login-form",
				RuleID:   "forms:login",
				Severity: "high",
				Locations: []model.EvidenceLocation{
					{Selector: "form#login", LineStart: intPtr(2), LineEnd: intPtr(4), Confidence: floatPtr(1.0)},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := interfaces.Logger(nil)
	versionID := uuid.New().String()

	if err := ScoreAndAttributeVersion(context.Background(), db, logger, assr, model.ScoreOptions{RequestLocations: true}, versionID, parentVersionID, diffID, diffJSON, []byte(`<html><body><form id="login"><input name="username"/></form></body></html>`)); err != nil {
		t.Fatalf("ScoreAndAttributeVersion failed: %v", err)
	}

	// one version_evidence_locations row created
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM version_evidence_locations WHERE version_id = ?`, versionID); cnt < 1 {
		t.Fatalf("expected >=1 evidence location row, got %d", cnt)
	}

	// diff_attributions should have one row for diffID
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM diff_attributions WHERE diff_id = ?`, diffID); cnt != 1 {
		t.Fatalf("expected 1 diff_attributions row, got %d", cnt)
	}

	// verify chunk_index for the inserted row is 1 (the added form chunk)
	var chunkIdx int
	if err := db.QueryRow(`SELECT chunk_index FROM diff_attributions WHERE diff_id = ? LIMIT 1`, diffID).Scan(&chunkIdx); err != nil {
		t.Fatalf("query chunk_index failed: %v", err)
	}
	if chunkIdx != 1 {
		t.Fatalf("expected chunk_index 1, got %d", chunkIdx)
	}
}

// Test that evidence with multiple locations splits weights proportionally based on per-location confidence.
func TestScoreAndAttributeVersion_MultipleLocations_SplitsWeights(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// parent score
	parentVersionID := uuid.New().String()
	parentScoreID := uuid.New().String()
	parentScoreJSON := `{"score":0.1}`
	if _, err := db.Exec(`INSERT INTO version_scores (id, version_id, scoring_version, score, normalized, confidence, score_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		parentScoreID, parentVersionID, "v-base", 0.1, 10, 0.6, parentScoreJSON, time.Now().Unix()); err != nil {
		t.Fatalf("insert parent score: %v", err)
	}

	// diff with two chunks matched by selectors .a and .b
	diffID := uuid.New().String()
	diffJSON := `{
		"body_diff": {
			"chunks": [
				{"type":"added","content":"<div class=\"a\">one</div>"},
				{"type":"added","content":"<div class=\"b\">two</div>"}
			]
		}
	}`

	// evidence with two locations, confidences 1.0 and 0.5 (expected weights 2:1)
	score := &model.ScoreResult{
		Score:      0.6,
		Normalized: 60,
		Confidence: 0.8,
		Version:    "v-test-3",
		Evidence: []model.EvidenceItem{
			{
				ID:       "ev-3",
				Key:      "repeated-pattern",
				RuleID:   "patterns:repeat",
				Severity: "medium",
				Locations: []model.EvidenceLocation{
					{Selector: ".a", Confidence: floatPtr(1.0)},
					{Selector: ".b", Confidence: floatPtr(0.5)},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := interfaces.Logger(nil)
	versionID := uuid.New().String()

	if err := ScoreAndAttributeVersion(context.Background(), db, logger, assr, model.ScoreOptions{RequestLocations: true}, versionID, parentVersionID, diffID, diffJSON, []byte(`<html><body><div class="a">one</div><div class="b">two</div></body></html>`)); err != nil {
		t.Fatalf("ScoreAndAttributeVersion failed: %v", err)
	}

	// Expect two diff_attributions rows for this diff
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM diff_attributions WHERE diff_id = ?`, diffID); cnt != 2 {
		t.Fatalf("expected 2 diff_attributions rows, got %d", cnt)
	}

	// read contributions to confirm ratio approx 2:1 (largest first)
	var w1, w2 float64
	if err := db.QueryRow(`SELECT contribution FROM diff_attributions WHERE diff_id = ? ORDER BY contribution DESC LIMIT 1 OFFSET 0`, diffID).Scan(&w1); err != nil {
		t.Fatalf("read contribution w1 failed: %v", err)
	}
	if err := db.QueryRow(`SELECT contribution FROM diff_attributions WHERE diff_id = ? ORDER BY contribution DESC LIMIT 1 OFFSET 1`, diffID).Scan(&w2); err != nil {
		t.Fatalf("read contribution w2 failed: %v", err)
	}
	if !(w1 > w2 && (w1/w2) > 1.7 && (w1/w2) < 2.3) {
		t.Fatalf("expected weight ratio ~2:1, got %v:%v (ratio %v)", w1, w2, w1/w2)
	}
}

// helpers
func intPtr(i int) *int       { return &i }
func floatPtr(f float64) *float64 { return &f }
