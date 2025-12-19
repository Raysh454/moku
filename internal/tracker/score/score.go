package score

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
	_ "modernc.org/sqlite"
)

type ScoreAttributer struct {
	assessor assessor.Assessor
	db       *sql.DB
	logger   logging.Logger
}

func New(assessor assessor.Assessor, db *sql.DB, logger logging.Logger) *ScoreAttributer {
	return &ScoreAttributer{
		assessor: assessor,
		db:       db,
		logger:   logger,
	}
}

func (sa *ScoreAttributer) ScoreAndAttribute(ctx context.Context, cr *models.CommitResult, opts *assessor.ScoreOptions) error {
	if sa.assessor == nil {
		return nil
	}

	scoreCtx, cancel := context.WithTimeout(ctx, opts.Timeout*time.Second)
	defer cancel()

	for _, s := range cr.Snapshots {
		urlTools, err := utils.NewURLTools(s.URL)
		if err != nil {
			if sa.logger != nil {
				sa.logger.Warn("failed to parse URL for scoring", logging.Field{Key: "url", Value: s.URL}, logging.Field{Key: "error", Value: err})
			}
			continue
		}
		filePath := urlTools.GetPath()

		scoreRes, err := sa.assessor.ScoreHTML(scoreCtx, s.Body, fmt.Sprintf("version:%s", cr.Version.ID), s.ID, filePath)
		if err != nil {
			if sa.logger != nil {
				sa.logger.Warn("scoring failed", logging.Field{Key: "version_id", Value: cr.Version.ID}, logging.Field{Key: "error", Value: err})
			}
			continue
		}

		// TODO: Call ScoreHeaders when implemented.

		if err := sa.attributeScore(ctx, scoreRes, cr.Version.ID); err != nil {
			if sa.logger != nil {
				sa.logger.Warn("attributeScore failed", logging.Field{Key: "version_id", Value: cr.Version.ID}, logging.Field{Key: "error", Value: err})
			}
		}
	}

	return nil
}

func (sa *ScoreAttributer) attributeScore(ctx context.Context, scoreRes *assessor.ScoreResult, versionID string) error {
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
		  (id, version_id, score, normalized, confidence, scoring_version, created_at, score_json, matched_rules, meta, raw_features)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, scoreID, versionID, scoreRes.Score, scoreRes.Normalized, scoreRes.Confidence, scoreRes.Version, time.Now().Unix(), string(scoreJSON), string(matchedRulesJSON), string(metaJSON), string(rawFeaturesJSON)); err != nil {
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

func (sa *ScoreAttributer) insertEvidenceItems(ctx context.Context, tx *sql.Tx, scoreID string, items []assessor.EvidenceItem) error {
	for _, item := range items {
		evidenceID := uuid.New().String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_items
			  (id, score_result_id, evidence_uid, item_key, rule_id, severity, description, value)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, evidenceID, scoreID, item.ID, item.Key, item.RuleID, item.Severity, item.Description, func() string {
			valBytes, err := json.Marshal(item.Value)
			if err != nil {
				return "{}"
			}
			return string(valBytes)
		}()); err != nil {
			return fmt.Errorf("insert evidence item: %w", err)
		}

		// Insert locations
		if err := sa.insertEvidenceLocations(ctx, tx, evidenceID, item.Locations); err != nil {
			return fmt.Errorf("insert evidence locations: %w", err)
		}
	}

	return nil
}

func (sa *ScoreAttributer) insertEvidenceLocations(ctx context.Context, tx *sql.Tx, evidenceID string, locations []assessor.EvidenceLocation) error {
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
			  (evidence_item_id, snapshot_id, css_selector, regex_pattern, file_path, byte_start, byte_end, line_start, line_end, note)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, evidenceID, loc.SnapshotID, loc.Selector, loc.RegexPattern, loc.FilePath, toI64(loc.ByteStart), toI64(loc.ByteEnd), toI64(loc.LineStart), toI64(loc.LineEnd), loc.Note); err != nil {
			return fmt.Errorf("insert evidence location: %w", err)
		}
	}

	return nil
}
