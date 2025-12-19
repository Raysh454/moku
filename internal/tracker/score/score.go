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

func (sa *SQLiteScoreTracker) ScoreAndAttribute(ctx context.Context, cr *models.CommitResult, opts *assessor.ScoreOptions) error {
	if sa.assessor == nil {
		return nil
	}

	scoreCtx, cancel := context.WithTimeout(ctx, opts.Timeout*time.Second)
	defer cancel()

	for _, s := range cr.Snapshots {
		scoreRes, err := sa.assessor.ScoreSnapshot(scoreCtx, s, cr.Version.ID)
		if err != nil {
			if sa.logger != nil {
				sa.logger.Warn("scoring failed", logging.Field{Key: "version_id", Value: cr.Version.ID}, logging.Field{Key: "error", Value: err})
			}
			continue
		}

		if err := sa.attributeScore(ctx, scoreRes, s.ID, cr.Version.ID, s.URL); err != nil {
			if sa.logger != nil {
				sa.logger.Warn("attributeScore failed", logging.Field{Key: "version_id", Value: cr.Version.ID}, logging.Field{Key: "error", Value: err})
			}
		}
	}

	return nil
}

func (sa *SQLiteScoreTracker) attributeScore(ctx context.Context, scoreRes *assessor.ScoreResult, snapshotID, versionID, url string) error {
	scoreJSON, err := json.Marshal(scoreRes)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("failed to marshal score result", logging.Field{Key: "err", Value: err})
		}
		scoreJSON = []byte("{}")
	}

	matchedRulesJSON, err := json.Marshal(scoreRes.MatchedRules)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("failed to marshal matched rules", logging.Field{Key: "err", Value: err})
		}
		matchedRulesJSON = []byte("{}")
	}

	metaJSON, err := json.Marshal(scoreRes.Meta)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("failed to marshal meta", logging.Field{Key: "err", Value: err})
		}
		metaJSON = []byte("{}")
	}

	rawFeaturesJSON, err := json.Marshal(scoreRes.RawFeatures)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("failed to marshal raw features", logging.Field{Key: "err", Value: err})
		}
		rawFeaturesJSON = []byte("{}")
	}

	tx, err := sa.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rb := tx.Rollback(); rb != nil && rb != sql.ErrTxDone {
			if sa.logger != nil {
				sa.logger.Warn("rollback failed", logging.Field{Key: "err", Value: rb})
			}
		}
	}()

	scoreID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO score_results
		  (id, snapshot_id, version_id, url, score, normalized, confidence, scoring_version, created_at, score_json, matched_rules, meta, raw_features)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, scoreID, snapshotID, versionID, url, scoreRes.Score, scoreRes.Normalized, scoreRes.Confidence, scoreRes.Version, time.Now().Unix(), string(scoreJSON), string(matchedRulesJSON), string(metaJSON), string(rawFeaturesJSON)); err != nil {
		return fmt.Errorf("insert score_results: %w", err)
	}

	// Insert evidence items and locations
	if err = sa.insertEvidenceItems(ctx, tx, scoreID, scoreRes.Evidence); err != nil {
		return fmt.Errorf("insert evidence items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (sa *SQLiteScoreTracker) insertEvidenceItems(ctx context.Context, tx *sql.Tx, scoreID string, items []assessor.EvidenceItem) error {
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

		// Insert locations
		if err := sa.insertEvidenceLocations(ctx, tx, evidenceID, item.Locations); err != nil {
			return fmt.Errorf("insert evidence locations: %w", err)
		}
	}

	return nil
}

func (sa *SQLiteScoreTracker) insertEvidenceLocations(ctx context.Context, tx *sql.Tx, evidenceID string, locations []assessor.EvidenceLocation) error {
	// helper to convert *int to nullable int64
	toI64 := func(p *int) any {
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
			toI64(loc.ParentDOMIndex), toI64(loc.DOMIndex), toI64(loc.ByteStart), toI64(loc.ByteEnd), toI64(loc.LineStart), toI64(loc.LineEnd),
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
func (s *SQLiteScoreTracker) GetScoreResultFromSnapshotID(ctx context.Context, snapshotID string) (*assessor.ScoreResult, error) {
	var scoreJSON string

	err := s.db.QueryRowContext(ctx, `
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
func (s *SQLiteScoreTracker) GetScoreResultsFromVersionID(ctx context.Context, versionID string) ([]*assessor.ScoreResult, error) {
	rows, err := s.db.QueryContext(ctx, `
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

		var sr assessor.ScoreResult
		if err := json.Unmarshal([]byte(scoreJSON), &sr); err != nil {
			return nil, fmt.Errorf("unmarshal score result: %w", err)
		}

		results = append(results, &sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate score_results: %w", err)
	}

	return results, nil
}

// GetSecurityDiff retrieves a detailed SecurityDiff between two snapshots.
// Enforces that both snapshots refer to the same URL/file (if both exist).
func (s *SQLiteScoreTracker) GetSecurityDiff(ctx context.Context, baseSnapshotID, headSnapshotID string) (*assessor.SecurityDiff, error) {
	if headSnapshotID == "" || baseSnapshotID == "" {
		return nil, errors.New("headSnapshotID/baseSnapshotID is required")
	}

	baseScore, err := s.GetScoreResultFromSnapshotID(ctx, baseSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("get base score result: %w", err)
	}

	headScore, err := s.GetScoreResultFromSnapshotID(ctx, headSnapshotID)
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
func (s *SQLiteScoreTracker) GetSecurityDiffOverview(ctx context.Context, baseID, headID string) (*assessor.SecurityDiffOverview, error) {
	if headID == "" || baseID == "" {
		return nil, errors.New("headID/baseID is required")
	}

	// Get all score results for head version.
	headScores, err := s.GetScoreResultsFromVersionID(ctx, headID)
	if err != nil {
		return nil, fmt.Errorf("get head scores: %w", err)
	}

	// Get all score results for base version.
	baseScores, err := s.GetScoreResultsFromVersionID(ctx, baseID)
	if err != nil {
		return nil, fmt.Errorf("get base scores: %w", err)
	}

	// Index base and head by URL for easy matching.
	baseByURL := make(map[string]*assessor.ScoreResult)
	for _, sr := range baseScores {
		if sr == nil {
			continue
		}
		baseByURL[sr.AttackSurface.URL] = sr
	}

	headByURL := make(map[string]*assessor.ScoreResult)
	for _, sr := range headScores {
		if sr == nil {
			continue
		}
		headByURL[sr.AttackSurface.URL] = sr
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
		var baseAS, headAS *attacksurface.AttackSurface
		if baseScore != nil {
			baseAS = baseScore.AttackSurface
		}
		if headScore != nil {
			headAS = headScore.AttackSurface
		}

		sd, err := assessor.NewSecurityDiff(
			url,
			baseID,
			headID,
			baseSnapshotID,
			headSnapshotID,
			baseScore,
			headScore,
			baseAS,
			headAS,
		)

		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to build security diff for url",
					logging.Field{Key: "url", Value: url},
					logging.Field{Key: "err", Value: err},
				)
			}
			continue
		}
		diffs = append(diffs, sd)
	}

	return assessor.NewSecurityDiffOverview(diffs), nil
}
