package indexer

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
)

type EndpointIndex interface {
	AddEndpoints(ctx context.Context, rawUrls []string, source string) ([]string, error)
	ListEndpoints(ctx context.Context, status string, limit int) ([]Endpoint, error)
	MarkPending(ctx context.Context, canonical string) error
	MarkPendingBatch(ctx context.Context, canonicals []string) error
	MarkFetched(ctx context.Context, canonical string, versionID string, fetchedAt time.Time) error
	MarkFetchedBatch(ctx context.Context, canonicals []string, versionID string, fetchedAt time.Time) error
	MarkFailed(ctx context.Context, canonical string, reason string) error
}

var _ EndpointIndex = (*Index)(nil)

// Index persists and queries discovered endpoints in the site DB.
type Index struct {
	db     *sql.DB
	logger logging.Logger
	// canonicalization options can be made configurable later
	canonOpts utils.CanonicalizeOptions
}

type Endpoint struct {
	ID                 string
	RawURL             string
	CanonicalURL       string
	Host               string
	Path               string
	FirstDiscoveredAt  int64
	LastDiscoveredAt   int64
	LastFetchedVersion string
	LastFetchedAt      int64
	Status             string
	Source             string
	Meta               string
}

func NewIndex(db *sql.DB, logger logging.Logger, opts utils.CanonicalizeOptions) *Index {
	return &Index{db: db, logger: logger, canonOpts: opts}
}

// AddEndpoints canonicalizes and inserts endpoints. Returns newly created canonical URLs.
func (ix *Index) AddEndpoints(ctx context.Context, rawUrls []string, source string) ([]string, error) {
	now := time.Now().Unix()

	// Single transaction for the batch to keep behavior consistent.
	tx, err := ix.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Properly check Rollback error to satisfy errcheck/vet.
	// If Commit() succeeds, Rollback() will return sql.ErrTxDone which we can ignore.
	defer func() {
		if rerr := tx.Rollback(); rerr != nil && rerr != sql.ErrTxDone {
			if ix.logger != nil {
				ix.logger.Warn("index: tx rollback failed",
					logging.Field{Key: "error", Value: rerr})
			}
		}
	}()

	// Prepare an INSERT OR IGNORE statement; we will check RowsAffected to know if
	// the insert created a row.
	stmtInsert, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO endpoints
		(id, raw_url, canonical_url, host, path, first_discovered_at, last_discovered_at, status, discovery_source, meta)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmtInsert.Close()

	// Prepared statement to update the last_discovered_at for existing rows.
	stmtUpdateLastDiscovered, err := tx.PrepareContext(ctx, `UPDATE endpoints SET last_discovered_at = ? WHERE canonical_url = ?`)
	if err != nil {
		return nil, err
	}
	defer stmtUpdateLastDiscovered.Close()

	newCanonicals := make([]string, 0, len(rawUrls))
	for _, ru := range rawUrls {
		canon, err := utils.Canonicalize(ru, ix.canonOpts)
		if err != nil {
			if ix.logger != nil {
				ix.logger.Warn("index: canonicalize failed", logging.Field{Key: "url", Value: ru}, logging.Field{Key: "err", Value: err})
			}
			continue
		}

		u, err := utils.NewURLTools(canon)
		if err != nil {
			if ix.logger != nil {
				ix.logger.Warn("index: NewURLTools failed", logging.Field{Key: "url", Value: canon}, logging.Field{Key: "err", Value: err})
			}
			continue
		}
		host := u.URL.Hostname()
		path := u.URL.Path

		id := uuid.New().String()
		res, err := stmtInsert.ExecContext(ctx, id, ru, canon, host, path, now, now, "new", source, "{}")
		if err != nil {
			return nil, err
		}
		ra, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if ra > 0 {
			// A new row was inserted.
			newCanonicals = append(newCanonicals, canon)
			continue
		}

		// Insert was ignored (row already exists). Update last_discovered_at for existing row.
		if _, err := stmtUpdateLastDiscovered.ExecContext(ctx, now, canon); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return newCanonicals, nil
}

func (ix *Index) MarkPending(ctx context.Context, canonical string) error {
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET status = ? WHERE canonical_url = ?`, "pending", canonical)
	return err
}

func (ix *Index) MarkPendingBatch(ctx context.Context, canonicals []string) error {
	if len(canonicals) == 0 {
		return nil
	}

	// Build placeholders: ?, ?, ?, ...
	placeholders := make([]string, len(canonicals))
	args := make([]any, 0, 1+len(canonicals))
	args = append(args, "pending") // first arg is status
	for i, c := range canonicals {
		placeholders[i] = "?"
		args = append(args, c)
	}

	q := `
		UPDATE endpoints
		SET status = ?
		WHERE canonical_url IN (` + strings.Join(placeholders, ",") + `)
	`
	_, err := ix.db.ExecContext(ctx, q, args...)
	return err
}

func (ix *Index) MarkFetched(ctx context.Context, canonical string, versionID string, fetchedAt time.Time) error {
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET last_fetched_version = ?, last_fetched_at = ?, status = ? WHERE canonical_url = ?`, versionID, fetchedAt.Unix(), "fetched", canonical)
	return err
}

func (ix *Index) MarkFetchedBatch(ctx context.Context, canonicals []string, versionID string, fetchedAt time.Time) error {
	if len(canonicals) == 0 {
		return nil
	}

	placeholders := make([]string, len(canonicals))
	args := make([]any, 0, 3+len(canonicals))
	args = append(args, versionID, fetchedAt.Unix(), "fetched")
	for i, c := range canonicals {
		placeholders[i] = "?"
		args = append(args, c)
	}

	q := `
		UPDATE endpoints
		SET last_fetched_version = ?,
			last_fetched_at     = ?,
			status              = ?
		WHERE canonical_url IN (` + strings.Join(placeholders, ",") + `)
	`
	_, err := ix.db.ExecContext(ctx, q, args...)
	return err
}

func (ix *Index) MarkFailed(ctx context.Context, canonical string, reason string) error {
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET status = ?, meta = json_set(COALESCE(meta, '{}'), '$.last_error', ?) WHERE canonical_url = ?`, "failed", reason, canonical)
	return err
}

// ListEndpoints with simple filters; extend filter struct as needed.
func (ix *Index) ListEndpoints(ctx context.Context, status string, limit int) ([]Endpoint, error) {
	q := `SELECT id, raw_url, canonical_url, host, path, first_discovered_at, last_discovered_at, last_fetched_version, last_fetched_at, status, discovery_source, meta FROM endpoints`
	var rows *sql.Rows
	var err error
	if status != "" {
		q += ` WHERE status = ? ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, status, limit)
	} else {
		q += ` ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Endpoint{}
	for rows.Next() {
		var e Endpoint
		var lastFetchedAt sql.NullInt64
		var lastFetchedVersion sql.NullString
		// Scan into Null types for nullable columns to avoid scan errors.
		if err := rows.Scan(&e.ID, &e.RawURL, &e.CanonicalURL, &e.Host, &e.Path, &e.FirstDiscoveredAt, &e.LastDiscoveredAt, &lastFetchedVersion, &lastFetchedAt, &e.Status, &e.Source, &e.Meta); err != nil {
			return nil, err
		}
		if lastFetchedAt.Valid {
			e.LastFetchedAt = lastFetchedAt.Int64
		}
		if lastFetchedVersion.Valid {
			e.LastFetchedVersion = lastFetchedVersion.String
		}
		out = append(out, e)
	}
	return out, nil
}
