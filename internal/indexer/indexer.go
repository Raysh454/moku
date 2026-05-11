package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
)

type EndpointIndex interface {
	AddEndpoints(ctx context.Context, rawUrls []string, source string) ([]string, error)
	ListEndpoints(ctx context.Context, status string, limit int) ([]Endpoint, error)
	ListEndpointsFiltered(ctx context.Context, status string, limit int, filterCfg *filter.Config) ([]Endpoint, error)
	MarkPending(ctx context.Context, canonical string) error
	MarkPendingBatch(ctx context.Context, canonicals []string) error
	MarkFetched(ctx context.Context, canonical string, versionID string, fetchedAt time.Time) error
	MarkFetchedBatch(ctx context.Context, canonicals []string, versionID string, fetchedAt time.Time) error
	MarkFailed(ctx context.Context, canonical string, reason string) error
	MarkFilteredBatch(ctx context.Context, canonicals []string, reasons map[string]string) error
	UnfilterBatch(ctx context.Context, canonicals []string) error
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
	ID                 string `json:"id"`
	RawURL             string `json:"url"`
	CanonicalURL       string `json:"canonical_url"`
	Host               string `json:"host"`
	Path               string `json:"path"`
	FirstDiscoveredAt  int64  `json:"first_discovered_at"`
	LastDiscoveredAt   int64  `json:"last_discovered_at"`
	LastFetchedVersion string `json:"last_fetched_version"`
	LastFetchedAt      int64  `json:"last_fetched_at"`
	Status             string `json:"status"`
	Source             string `json:"source"`
	Meta               string `json:"meta"`
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
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET status = ? WHERE canonical_url = ? AND status != 'filtered'`, "pending", canonical)
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
		AND status != 'filtered'
	`
	_, err := ix.db.ExecContext(ctx, q, args...)
	return err
}

func (ix *Index) MarkFetched(ctx context.Context, canonical string, versionID string, fetchedAt time.Time) error {
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET last_fetched_version = ?, last_fetched_at = ?, status = ? WHERE canonical_url = ? AND status != 'filtered'`, versionID, fetchedAt.Unix(), "fetched", canonical)
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
		AND status != 'filtered'
	`
	_, err := ix.db.ExecContext(ctx, q, args...)
	return err
}

func (ix *Index) MarkFailed(ctx context.Context, canonical string, reason string) error {
	_, err := ix.db.ExecContext(ctx, `UPDATE endpoints SET status = ?, meta = json_set(COALESCE(meta, '{}'), '$.last_error', ?) WHERE canonical_url = ?`, "failed", reason, canonical)
	return err
}

// ListEndpoints with simple filters; extend filter struct as needed.
// Limit of 0 means no limit.
// By default, excludes endpoints with status="filtered" unless:
// - status="all" (returns everything including filtered)
// - status="filtered" (returns only filtered endpoints)
func (ix *Index) ListEndpoints(ctx context.Context, status string, limit int) ([]Endpoint, error) {
	ix.logger.Debug("index: ListEndpoints called", logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
	if limit <= 0 {
		limit = -1
	}

	q := `SELECT id, raw_url, canonical_url, host, path, first_discovered_at, last_discovered_at, last_fetched_version, last_fetched_at, status, discovery_source, meta FROM endpoints`
	var rows *sql.Rows
	var err error

	switch status {
	case "*", "all":
		// Return everything including filtered
		q += ` ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, limit)
	case "filtered":
		// Return only filtered endpoints
		q += ` WHERE status = 'filtered' ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, limit)
	case "":
		// Default: return all non-filtered endpoints
		q += ` WHERE status != 'filtered' ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, limit)
	default:
		// Specific status (e.g., "pending", "fetched") - exclude filtered
		q += ` WHERE status = ? ORDER BY last_discovered_at DESC LIMIT ?`
		rows, err = ix.db.QueryContext(ctx, q, status, limit)
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
	ix.logger.Debug("index: ListEndpoints completed", logging.Field{Key: "returned_count", Value: len(out)})
	return out, nil
}

// ListEndpointsFiltered returns endpoints filtered by the given filter config.
// URLs that match the filter are marked as "filtered" with their reason and not returned.
func (ix *Index) ListEndpointsFiltered(ctx context.Context, status string, limit int, filterCfg *filter.Config) ([]Endpoint, error) {
	// First get all endpoints matching the status
	endpoints, err := ix.ListEndpoints(ctx, status, 0) // Get all, we'll limit after filtering
	if err != nil {
		return nil, err
	}

	// If no filter config, return as-is (with limit)
	if filterCfg == nil || filterCfg.IsEmpty() {
		if limit > 0 && len(endpoints) > limit {
			return endpoints[:limit], nil
		}
		return endpoints, nil
	}

	// Create filter engine
	engine := filter.NewEngine(filterCfg)

	// Separate filtered from unfiltered
	unfiltered := make([]Endpoint, 0, len(endpoints))
	filteredURLs := make([]string, 0)
	filteredReasons := make(map[string]string)

	for _, ep := range endpoints {
		result := engine.ShouldFilter(ep.CanonicalURL)
		if result.Filtered {
			filteredURLs = append(filteredURLs, ep.CanonicalURL)
			filteredReasons[ep.CanonicalURL] = result.Reason
		} else {
			unfiltered = append(unfiltered, ep)
		}
	}

	// Mark filtered URLs in the database (in batches for performance)
	if len(filteredURLs) > 0 {
		if err := ix.MarkFilteredBatch(ctx, filteredURLs, filteredReasons); err != nil {
			ix.logger.Warn("index: failed to mark filtered URLs",
				logging.Field{Key: "error", Value: err.Error()},
				logging.Field{Key: "count", Value: len(filteredURLs)})
		}
	}

	// Apply limit
	if limit > 0 && len(unfiltered) > limit {
		return unfiltered[:limit], nil
	}

	ix.logger.Debug("index: ListEndpointsFiltered completed",
		logging.Field{Key: "unfiltered_count", Value: len(unfiltered)},
		logging.Field{Key: "filtered_count", Value: len(filteredURLs)})

	return unfiltered, nil
}

// MarkFilteredBatch marks multiple endpoints as filtered with their reasons.
// Reasons map: canonical_url -> filter reason string
func (ix *Index) MarkFilteredBatch(ctx context.Context, canonicals []string, reasons map[string]string) error {
	if len(canonicals) == 0 {
		return nil
	}

	now := time.Now().Unix()

	// Process in batches of 500 to avoid SQLite parameter limits
	const batchSize = 500
	for i := 0; i < len(canonicals); i += batchSize {
		end := i + batchSize
		if end > len(canonicals) {
			end = len(canonicals)
		}
		batch := canonicals[i:end]

		tx, err := ix.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		stmt, err := tx.PrepareContext(ctx,
			`UPDATE endpoints 
			 SET status = 'filtered', 
			     meta = json_set(COALESCE(meta, '{}'), '$.filter_reason', ?, '$.filtered_at', ?)
			 WHERE canonical_url = ?`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		for _, canonical := range batch {
			reason := reasons[canonical]
			if reason == "" {
				reason = "unknown"
			}
			if _, err := stmt.ExecContext(ctx, reason, now, canonical); err != nil {
				_ = stmt.Close()
				_ = tx.Rollback()
				return err
			}
		}

		_ = stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// UnfilterBatch resets filtered endpoints back to "pending" status so they can be fetched.
func (ix *Index) UnfilterBatch(ctx context.Context, canonicals []string) error {
	if len(canonicals) == 0 {
		return nil
	}

	// Process in batches
	const batchSize = 500
	for i := 0; i < len(canonicals); i += batchSize {
		end := i + batchSize
		if end > len(canonicals) {
			end = len(canonicals)
		}
		batch := canonicals[i:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, 1+len(batch))
		args = append(args, "pending")
		for j, c := range batch {
			placeholders[j] = "?"
			args = append(args, c)
		}

		q := `
			UPDATE endpoints
			SET status = ?,
			    meta = json_remove(meta, '$.filter_reason', '$.filtered_at')
			WHERE canonical_url IN (` + strings.Join(placeholders, ",") + `)
			AND status = 'filtered'
		`
		if _, err := ix.db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}

	return nil
}

// GetEndpointStats returns counts of endpoints by status.
func (ix *Index) GetEndpointStats(ctx context.Context) (map[string]int, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT status, COUNT(*) as count FROM endpoints GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	total := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
		total += count
	}
	stats["total"] = total

	return stats, nil
}

// GetFilteredEndpointsByReason returns counts of filtered endpoints grouped by filter reason.
func (ix *Index) GetFilteredEndpointsByReason(ctx context.Context) (map[string]int, error) {
	rows, err := ix.db.QueryContext(ctx,
		`SELECT json_extract(meta, '$.filter_reason') as reason, COUNT(*) as count 
		 FROM endpoints 
		 WHERE status = 'filtered' AND json_extract(meta, '$.filter_reason') IS NOT NULL
		 GROUP BY reason`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var reason sql.NullString
		var count int
		if err := rows.Scan(&reason, &count); err != nil {
			return nil, err
		}
		if reason.Valid {
			stats[reason.String] = count
		}
	}

	return stats, nil
}

// filterMeta is a helper struct for JSON serialization of filter metadata
type filterMeta struct {
	FilterReason string `json:"filter_reason,omitempty"`
	FilteredAt   int64  `json:"filtered_at,omitempty"`
}

// GetFilteredEndpoints returns endpoints with status "filtered" along with their filter reasons.
func (ix *Index) GetFilteredEndpoints(ctx context.Context, limit int) ([]filter.FilteredEndpoint, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := ix.db.QueryContext(ctx,
		`SELECT raw_url, canonical_url, meta FROM endpoints WHERE status = 'filtered' LIMIT ?`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []filter.FilteredEndpoint
	for rows.Next() {
		var rawURL, canonicalURL string
		var metaJSON string
		if err := rows.Scan(&rawURL, &canonicalURL, &metaJSON); err != nil {
			return nil, err
		}

		var meta filterMeta
		if metaJSON != "" {
			_ = json.Unmarshal([]byte(metaJSON), &meta)
		}

		result = append(result, filter.FilteredEndpoint{
			URL:          rawURL,
			CanonicalURL: canonicalURL,
			FilterReason: meta.FilterReason,
			FilteredAt:   time.Unix(meta.FilteredAt, 0),
		})
	}

	return result, nil
}
