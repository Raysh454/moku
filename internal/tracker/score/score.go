package score

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	_ "modernc.org/sqlite"
)

type SQLiteScoreTracker struct {
	assessor assessor.Assessor
	db       *sql.DB
	logger   logging.Logger
}

func New(assessor assessor.Assessor, db *sql.DB, logger logging.Logger) *SQLiteScoreTracker {
	return &SQLiteScoreTracker{
		assessor: assessor,
		db:       db,
		logger:   logger,
	}
}

func (scoreTracker *SQLiteScoreTracker) ScoreAndAttribute(ctx context.Context, commitResult *models.CommitResult, scoreTimeout time.Duration) error {
	if scoreTracker.assessor == nil {
		return nil
	}

	scoreCtx, cancel := context.WithTimeout(ctx, scoreTimeout)
	defer cancel()

	for _, snapshot := range commitResult.Snapshots {
		scoreResult, err := scoreTracker.assessor.ScoreSnapshot(scoreCtx, snapshot, commitResult.Version.ID)
		if err != nil {
			if scoreTracker.logger != nil {
				scoreTracker.logger.Warn("scoring failed", logging.Field{Key: "version_id", Value: commitResult.Version.ID}, logging.Field{Key: "error", Value: err})
			}
			continue
		}

		if err := scoreTracker.attributeScore(ctx, scoreResult, snapshot.ID, commitResult.Version.ID, snapshot.URL); err != nil {
			if scoreTracker.logger != nil {
				scoreTracker.logger.Warn("attributeScore failed", logging.Field{Key: "version_id", Value: commitResult.Version.ID}, logging.Field{Key: "error", Value: err})
			}
		}
	}

	return nil
}

func (scoreTracker *SQLiteScoreTracker) attributeScore(ctx context.Context, scoreResult *assessor.ScoreResult, snapshotID, versionID, url string) error {
	scoreJSON, err := json.Marshal(scoreResult)
	if err != nil {
		if scoreTracker.logger != nil {
			scoreTracker.logger.Warn("failed to marshal score result", logging.Field{Key: "err", Value: err})
		}
		scoreJSON = []byte("{}")
	}

	matchedRulesJSON, err := json.Marshal(scoreResult.MatchedRules)
	if err != nil {
		if scoreTracker.logger != nil {
			scoreTracker.logger.Warn("failed to marshal matched rules", logging.Field{Key: "err", Value: err})
		}
		matchedRulesJSON = []byte("{}")
	}

	metaJSON, err := json.Marshal(scoreResult.Meta)
	if err != nil {
		if scoreTracker.logger != nil {
			scoreTracker.logger.Warn("failed to marshal meta", logging.Field{Key: "err", Value: err})
		}
		metaJSON = []byte("{}")
	}

	rawFeaturesJSON, err := json.Marshal(scoreResult.RawFeatures)
	if err != nil {
		if scoreTracker.logger != nil {
			scoreTracker.logger.Warn("failed to marshal raw features", logging.Field{Key: "err", Value: err})
		}
		rawFeaturesJSON = []byte("{}")
	}

	tx, err := scoreTracker.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rb := tx.Rollback(); rb != nil && rb != sql.ErrTxDone {
			if scoreTracker.logger != nil {
				scoreTracker.logger.Warn("rollback failed", logging.Field{Key: "err", Value: rb})
			}
		}
	}()

	scoreID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO score_results
		  (id, snapshot_id, version_id, url, score, normalized, confidence, scoring_version, created_at, score_json, matched_rules, meta, raw_features)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, scoreID, snapshotID, versionID, url, scoreResult.Score, scoreResult.Normalized, scoreResult.Confidence, scoreResult.Version, time.Now().Unix(), string(scoreJSON), string(matchedRulesJSON), string(metaJSON), string(rawFeaturesJSON)); err != nil {
		return fmt.Errorf("insert score_results: %w", err)
	}

	if err = scoreTracker.insertEvidenceItems(ctx, tx, scoreID, scoreResult.Evidence); err != nil {
		return fmt.Errorf("insert evidence items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (scoreTracker *SQLiteScoreTracker) insertEvidenceItems(ctx context.Context, tx *sql.Tx, scoreID string, items []assessor.EvidenceItem) error {
	for _, item := range items {
		evidenceID := uuid.New().String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_items
			  (id, score_result_id, evidence_uid, item_key, rule_id, severity, description, value, contribution)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, evidenceID, scoreID, item.ID, item.Key, item.RuleID, item.Severity, item.Description, func() string {
			valBytes, err := json.Marshal(item.Value)
			if err != nil {
				return "{}"
			}
			return string(valBytes)
		}(), item.Contribution); err != nil {
			return fmt.Errorf("insert evidence item: %w", err)
		}

		if err := scoreTracker.insertEvidenceLocations(ctx, tx, evidenceID, item.Locations); err != nil {
			return fmt.Errorf("insert evidence locations: %w", err)
		}
	}

	return nil
}

func (scoreTracker *SQLiteScoreTracker) insertEvidenceLocations(ctx context.Context, tx *sql.Tx, evidenceID string, locations []assessor.EvidenceLocation) error {
	toNullableInt64 := func(p *int) any {
		if p == nil {
			return nil
		}
		return int64(*p)
	}

	for _, loc := range locations {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_locations
			  (evidence_item_id, snapshot_id, location_type, css_selector, xpath, regex_pattern, file_path,
			   parent_dom_index, dom_index, byte_start, byte_end, line_start, line_end, line, column, header_name, cookie_name, parameter_name, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, evidenceID, loc.SnapshotID, loc.Type, loc.Selector, loc.XPath, loc.RegexPattern, loc.FilePath,
			toNullableInt64(loc.ParentDOMIndex), toNullableInt64(loc.DOMIndex), toNullableInt64(loc.ByteStart), toNullableInt64(loc.ByteEnd), toNullableInt64(loc.LineStart), toNullableInt64(loc.LineEnd),
			loc.Line, loc.Column, loc.HeaderName, loc.CookieName, loc.ParamName, loc.Note); err != nil {
			return fmt.Errorf("insert evidence location: %w", err)
		}
	}

	return nil
}

// GetScoreResultForVersion retrieves the ScoreResult for a given version ID.
// Returns nil, nil if no score exists for the version.
func GetScoreResultForVersion(ctx context.Context, db *sql.DB, versionID string) (*assessor.ScoreResult, error) {
	var scoreJSON string
	err := db.QueryRowContext(ctx, `
		SELECT score_json
		FROM score_results
		WHERE version_id = ?
		LIMIT 1
	`, versionID).Scan(&scoreJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query score_results: %w", err)
	}

	var scoreRes assessor.ScoreResult
	if err := json.Unmarshal([]byte(scoreJSON), &scoreRes); err != nil {
		return nil, fmt.Errorf("unmarshal score result: %w", err)
	}

	return &scoreRes, nil
}

// GetScoreResultFromSnapshotID retrieves the ScoreResult associated with a specific snapshot ID.
func (scoreTracker *SQLiteScoreTracker) GetScoreResultFromSnapshotID(ctx context.Context, snapshotID string) (*assessor.ScoreResult, error) {
	var scoreJSON string

	err := scoreTracker.db.QueryRowContext(ctx, `
		SELECT score_json
		FROM score_results
		WHERE snapshot_id = ?
		LIMIT 1
	`, snapshotID).Scan(&scoreJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query score_results: %w", err)
	}

	var scoreResult assessor.ScoreResult
	if err := json.Unmarshal([]byte(scoreJSON), &scoreResult); err != nil {
		return nil, fmt.Errorf("unmarshal score result: %w", err)
	}

	return &scoreResult, nil
}

// GetScoreResultsFromVersionID retrieves all ScoreResults associated with a specific version ID.
func (scoreTracker *SQLiteScoreTracker) GetScoreResultsFromVersionID(ctx context.Context, versionID string) ([]*assessor.ScoreResult, error) {
	rows, err := scoreTracker.db.QueryContext(ctx, `
		SELECT score_json
		FROM score_results
		WHERE version_id = ?
	`, versionID)
	if err != nil {
		return nil, fmt.Errorf("query score_results by version_id: %w", err)
	}
	defer rows.Close()

	var results []*assessor.ScoreResult

	for rows.Next() {
		var scoreJSON string
		if err := rows.Scan(&scoreJSON); err != nil {
			return nil, fmt.Errorf("scan score_json: %w", err)
		}

		var scoreResult assessor.ScoreResult
		if err := json.Unmarshal([]byte(scoreJSON), &scoreResult); err != nil {
			return nil, fmt.Errorf("unmarshal score result: %w", err)
		}

		results = append(results, &scoreResult)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate score_results: %w", err)
	}

	return results, nil
}

// GetSecurityDiff retrieves a detailed SecurityDiff between two snapshots.
// Enforces that both snapshots refer to the same URL/file (if both exist).
func (scoreTracker *SQLiteScoreTracker) GetSecurityDiff(ctx context.Context, baseSnapshotID, headSnapshotID string) (*assessor.SecurityDiff, error) {
	if headSnapshotID == "" || baseSnapshotID == "" {
		return nil, errors.New("headSnapshotID/baseSnapshotID is required")
	}

	baseScore, err := scoreTracker.GetScoreResultFromSnapshotID(ctx, baseSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("get base score result: %w", err)
	}

	headScore, err := scoreTracker.GetScoreResultFromSnapshotID(ctx, headSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("get head score result: %w", err)
	}

	if baseScore == nil {
		return nil, fmt.Errorf("no score result found for base snapshot %s", baseSnapshotID)
	}

	if headScore == nil {
		return nil, fmt.Errorf("no score result found for head snapshot %s", headSnapshotID)
	}

	if baseScore.AttackSurface.URL != headScore.AttackSurface.URL {
		return nil, fmt.Errorf("snapshot URL mismatch: base (%s) vs head (%s)", baseScore.AttackSurface.URL, headScore.AttackSurface.URL)
	}

	// Build SecurityDiff.
	diff, err := assessor.NewSecurityDiff(
		baseScore.AttackSurface.URL,
		baseScore.VersionID,
		headScore.VersionID,
		baseSnapshotID,
		headSnapshotID,
		baseScore,
		headScore,
		baseScore.AttackSurface,
		headScore.AttackSurface,
	)
	if err != nil {
		return nil, fmt.Errorf("build security diff: %w", err)
	}

	return diff, nil
}

// GetSecurityDiffOverview computes a security-focused diff overview between two versions.
func (scoreTracker *SQLiteScoreTracker) GetSecurityDiffOverview(ctx context.Context, baseID, headID string) (*assessor.SecurityDiffOverview, error) {
	if headID == "" || baseID == "" {
		return nil, errors.New("headID/baseID is required")
	}

	// Get all score results for head version.
	headScores, err := scoreTracker.GetScoreResultsFromVersionID(ctx, headID)
	if err != nil {
		return nil, fmt.Errorf("get head scores: %w", err)
	}

	// Get all score results for base version.
	baseScores, err := scoreTracker.GetScoreResultsFromVersionID(ctx, baseID)
	if err != nil {
		return nil, fmt.Errorf("get base scores: %w", err)
	}

	// Index base and head by URL for easy matching.
	baseByURL := make(map[string]*assessor.ScoreResult)
	for _, scoreResult := range baseScores {
		if scoreResult == nil {
			continue
		}
		baseByURL[scoreResult.AttackSurface.URL] = scoreResult
	}

	headByURL := make(map[string]*assessor.ScoreResult)
	for _, scoreResult := range headScores {
		if scoreResult == nil {
			continue
		}
		headByURL[scoreResult.AttackSurface.URL] = scoreResult
	}

	// Build a set of all URLs present in base or head.
	allURLs := make(map[string]struct{})
	for u := range headByURL {
		allURLs[u] = struct{}{}
	}
	for u := range baseByURL {
		allURLs[u] = struct{}{}
	}

	// For each URL, build a SecurityDiff and collect them.
	var diffs []*assessor.SecurityDiff
	for url := range allURLs {
		baseScore := baseByURL[url]
		headScore := headByURL[url]

		var baseSnapshotID, headSnapshotID string
		if baseScore != nil {
			baseSnapshotID = baseScore.SnapshotID
		}
		if headScore != nil {
			headSnapshotID = headScore.SnapshotID
		}

		// AttackSurfaces (if stored inside ScoreResult).
		var baseAttackSurface, headAttackSurface *attacksurface.AttackSurface
		if baseScore != nil {
			baseAttackSurface = baseScore.AttackSurface
		}
		if headScore != nil {
			headAttackSurface = headScore.AttackSurface
		}

		securityDiff, err := assessor.NewSecurityDiff(
			url,
			baseID,
			headID,
			baseSnapshotID,
			headSnapshotID,
			baseScore,
			headScore,
			baseAttackSurface,
			headAttackSurface,
		)

		if err != nil {
			if scoreTracker.logger != nil {
				scoreTracker.logger.Warn("failed to build security diff for url",
					logging.Field{Key: "url", Value: url},
					logging.Field{Key: "err", Value: err},
				)
			}
			continue
		}
		diffs = append(diffs, securityDiff)
	}

	return assessor.NewSecurityDiffOverview(diffs), nil
}
