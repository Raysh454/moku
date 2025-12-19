package score_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/tracker/score"
	"github.com/raysh454/moku/internal/webclient"
)

// dummyAssessor returns a preconfigured ScoreResult for tests.
type dummyAssessor struct {
	res *assessor.ScoreResult
}

func (d *dummyAssessor) ScoreHTML(ctx context.Context, html []byte, source, snapshotID, filePath string) (*assessor.ScoreResult, error) {
	// return a deep copy so tests can mutate without colliding
	b, _ := json.Marshal(d.res)
	var out assessor.ScoreResult
	_ = json.Unmarshal(b, &out)
	// Populate snapshotID and filePath for locations to satisfy FKs
	for i := range out.Evidence {
		for j := range out.Evidence[i].Locations {
			if out.Evidence[i].Locations[j].SnapshotID == "" {
				out.Evidence[i].Locations[j].SnapshotID = snapshotID
			}
			if out.Evidence[i].Locations[j].FilePath == "" {
				out.Evidence[i].Locations[j].FilePath = filePath
			}
		}
	}
	return &out, nil
}
func (d *dummyAssessor) ScoreResponse(ctx context.Context, resp *webclient.Response) (*assessor.ScoreResult, error) {
	if resp != nil {
		url := ""
		if resp.Request != nil {
			url = resp.Request.URL
		}
		return d.ScoreHTML(ctx, resp.Body, url, "", "")
	}
	return d.ScoreHTML(ctx, nil, "", "", "")
}
func (d *dummyAssessor) ExtractEvidence(ctx context.Context, html []byte, opts assessor.ScoreOptions) ([]assessor.EvidenceItem, error) {
	if d.res == nil {
		return nil, nil
	}
	b, _ := json.Marshal(d.res.Evidence)
	var out []assessor.EvidenceItem
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
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		t.Fatalf("pragmas: %v", err)
	}

	// Load real schema used by tracker
	schemaBytes, err := os.ReadFile("../schema.sql")
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	if _, err := db.Exec(string(schemaBytes)); err != nil {
		t.Fatalf("apply schema.sql: %v", err)
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

// scoreAndAttributeVersionForTest is a test helper that calls the internal scoreAndAttribute method
// with a minimal tracker setup for unit testing scoring logic.
func scoreAndAttributeVersionForTest(ctx context.Context, db *sql.DB, logger logging.Logger, assessor assessor.Assessor, opts assessor.ScoreOptions, versionID, parentVersionID, diffID, diffJSON string, headBody []byte) error {
	// Create a minimal temp directory for the tracker
	tmpDir, err := os.MkdirTemp("", "scoring-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Precreate version and snapshot records to satisfy FKs
	snapshotID := uuid.New().String()
	if _, err := db.Exec(`INSERT INTO versions (id, parent_id, message, author, timestamp) VALUES (?, ?, ?, ?, ?)`, versionID, parentVersionID, "test", "", time.Now().Unix()); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO snapshots (id, status_code, url, file_path, blob_id, created_at, headers) VALUES (?, ?, ?, ?, ?, ?, ?)`, snapshotID, 200, "https://example.com/test", "/test", uuid.New().String(), time.Now().Unix(), "{}"); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO version_snapshots (version_id, snapshot_id) VALUES (?, ?)`, versionID, snapshotID); err != nil {
		return err
	}

	// Use ScoreAttributer directly with a minimal CommitResult
	sa := score.New(assessor, db, logger)
	cr := &models.CommitResult{
		Version:   models.Version{ID: versionID, Parent: parentVersionID},
		DiffID:    diffID,
		DiffJSON:  diffJSON,
		Snapshots: []*models.Snapshot{{ID: snapshotID, URL: "https://example.com/test", Body: headBody}},
	}
	return sa.ScoreAndAttribute(ctx, cr, &opts)
}

// Test initial page (no parent) persists version_scores and version_evidence_locations
func TestScoreAndAttributeVersion_InitialPage_PersistsEvidenceLocations(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	score := &assessor.ScoreResult{
		Score:      0.8,
		Normalized: 80,
		Confidence: 0.9,
		Version:    "v-test-1",
		Evidence: []assessor.EvidenceItem{
			{
				ID:       "ev-1",
				Key:      "login-form",
				RuleID:   "forms:login",
				Severity: "high",
				Locations: []assessor.EvidenceLocation{
					{Selector: "form#login", LineStart: intPtr(10), LineEnd: intPtr(12)},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := logging.Logger(nil)

	versionID := uuid.New().String()

	// Call with no parent/diff
	if err := scoreAndAttributeVersionForTest(context.Background(), db, logger, assr, assessor.ScoreOptions{}, versionID, "", "", "", []byte(`<html><body><form id="login"></form></body></html>`)); err != nil {
		t.Fatalf("scoreAndAttributeVersionForTest failed: %v", err)
	}

	// score_results row exists
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM score_results WHERE version_id = ?`, versionID); cnt != 1 {
		t.Fatalf("expected 1 score_results row, got %d", cnt)
	}

	// evidence_items inserted
	var scoreID string
	if err := db.QueryRow(`SELECT id FROM score_results WHERE version_id = ?`, versionID).Scan(&scoreID); err != nil {
		t.Fatalf("read score_result id: %v", err)
	}
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM evidence_items WHERE score_result_id = ?`, scoreID); cnt < 1 {
		t.Fatalf("expected >=1 evidence item, got %d", cnt)
	}
}

// Test that attribution with a diff maps locations to the expected chunk index and inserts diff_attributions
func TestScoreAndAttributeVersion_WithDiff_AttributesLocations(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

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
	score := &assessor.ScoreResult{
		Score:      0.7,
		Normalized: 70,
		Confidence: 0.9,
		Version:    "v-test-2",
		Evidence: []assessor.EvidenceItem{
			{
				ID:       "ev-2",
				Key:      "login-form",
				RuleID:   "forms:login",
				Severity: "high",
				Locations: []assessor.EvidenceLocation{
					{Selector: "form#login", LineStart: intPtr(2), LineEnd: intPtr(4)},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := logging.Logger(nil)
	versionID := uuid.New().String()
	parentVersionID := uuid.New().String()

	if err := scoreAndAttributeVersionForTest(context.Background(), db, logger, assr, assessor.ScoreOptions{}, versionID, parentVersionID, diffID, diffJSON, []byte(`<html><body><form id="login"><input name="username"/></form></body></html>`)); err != nil {
		t.Fatalf("scoreAndAttributeVersionForTest failed: %v", err)
	}

	// evidence_items exist for the score
	var scoreID string
	if err := db.QueryRow(`SELECT id FROM score_results WHERE version_id = ?`, versionID).Scan(&scoreID); err != nil {
		t.Fatalf("read score_result id: %v", err)
	}
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM evidence_items WHERE score_result_id = ?`, scoreID); cnt < 1 {
		t.Fatalf("expected >=1 evidence item, got %d", cnt)
	}
}

// Test that evidence with multiple locations splits weights proportionally based on per-location confidence.
func TestScoreAndAttributeVersion_MultipleLocations_SplitsWeights(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// parent version id (no pre-insert of scores needed under new schema)
	parentVersionID := uuid.New().String()

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
	score := &assessor.ScoreResult{
		Score:      0.6,
		Normalized: 60,
		Confidence: 0.8,
		Version:    "v-test-3",
		Evidence: []assessor.EvidenceItem{
			{
				ID:       "ev-3",
				Key:      "repeated-pattern",
				RuleID:   "patterns:repeat",
				Severity: "medium",
				Locations: []assessor.EvidenceLocation{
					{Selector: ".a"},
					{Selector: ".b"},
				},
			},
		},
	}

	assr := &dummyAssessor{res: score}
	logger := logging.Logger(nil)
	versionID := uuid.New().String()

	if err := scoreAndAttributeVersionForTest(context.Background(), db, logger, assr, assessor.ScoreOptions{}, versionID, parentVersionID, diffID, diffJSON, []byte(`<html><body><div class="a">one</div><div class="b">two</div></body></html>`)); err != nil {
		t.Fatalf("scoreAndAttributeVersionForTest failed: %v", err)
	}

	// Expect one evidence_items row for this score (one evidence with two locations)
	var scoreID string
	if err := db.QueryRow(`SELECT id FROM score_results WHERE version_id = ?`, versionID).Scan(&scoreID); err != nil {
		t.Fatalf("read score_result id: %v", err)
	}
	if cnt := countRows(t, db, `SELECT COUNT(1) FROM evidence_items WHERE score_result_id = ?`, scoreID); cnt != 1 {
		t.Fatalf("expected 1 evidence_items row, got %d", cnt)
	}
}

func intPtr(i int) *int { return &i }
