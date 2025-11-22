package tracker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
	_ "modernc.org/sqlite" // SQLite driver
)

// SQLiteTracker implements interfaces.Tracker using SQLite for metadata storage
// and a content-addressed blob store for file content.
type SQLiteTracker struct {
	siteDir string
	db      *sql.DB
	store   *FSStore
	logger  interfaces.Logger
}

// NewSQLiteTracker creates a new SQLiteTracker instance.
// It initializes the SQLite database at siteDir/.moku/moku.db and sets up the blob store.
func NewSQLiteTracker(siteDir string, logger interfaces.Logger) (*SQLiteTracker, error) {
	if logger == nil {
		return nil, errors.New("tracker: nil logger provided")
	}

	// Ensure .moku directory exists
	mokuDir := filepath.Join(siteDir, ".moku")
	if err := os.MkdirAll(mokuDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .moku directory: %w", err)
	}

	// Open SQLite database
	dbPath := filepath.Join(mokuDir, "moku.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply schema and set pragmas
	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	// Create FSStore for blob storage
	blobsDir := filepath.Join(mokuDir, "blobs")
	store, err := NewFSStore(blobsDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create FSStore: %w", err)
	}

	logger.Info("SQLiteTracker initialized", interfaces.Field{Key: "siteDir", Value: siteDir})

	return &SQLiteTracker{
		siteDir: siteDir,
		db:      db,
		store:   store,
		logger:  logger,
	}, nil
}

// Ensure SQLiteTracker implements interfaces.Tracker at compile-time.
var _ interfaces.Tracker = (*SQLiteTracker)(nil)

// Commit stores a snapshot and returns a Version record representing the commit.
func (t *SQLiteTracker) Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.Version, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot cannot be nil")
	}
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	t.logger.Debug("Starting commit", 
		interfaces.Field{Key: "url", Value: snapshot.URL},
		interfaces.Field{Key: "message", Value: message})

	// Step 1: Store snapshot body as a blob
	blobID, err := t.store.Put(snapshot.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to store snapshot body: %w", err)
	}
	t.logger.Debug("Stored snapshot body", interfaces.Field{Key: "blobID", Value: blobID})

	// Step 2: Generate IDs
	snapshotID := uuid.New().String()
	versionID := uuid.New().String()
	timestamp := time.Now().Unix()
	if !snapshot.CreatedAt.IsZero() {
		timestamp = snapshot.CreatedAt.Unix()
	}

	// Step 3: Get parent version (current HEAD)
	parentID, err := t.readHEAD()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read HEAD: %w", err)
	}

	// Step 4: Begin transaction
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// Step 5: Insert snapshot record
	filePath := "index.html" // Default file path, could be derived from URL
	if snapshot.URL != "" {
		// TODO: Derive file path from URL (e.g., parse path component)
		filePath = "index.html"
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (id, url, file_path, created_at)
		VALUES (?, ?, ?, ?)
	`, snapshotID, snapshot.URL, filePath, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	// Step 6: Insert version record
	_, err = tx.ExecContext(ctx, `
		INSERT INTO versions (id, parent_id, snapshot_id, message, author, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, versionID, nullableString(parentID), snapshotID, message, nullableString(author), timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to insert version: %w", err)
	}

	// Step 7: Insert version_files record
	_, err = tx.ExecContext(ctx, `
		INSERT INTO version_files (version_id, file_path, blob_id, size)
		VALUES (?, ?, ?, ?)
	`, versionID, filePath, blobID, len(snapshot.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to insert version_files: %w", err)
	}

	// Step 8: Compute and store diff (if parent exists)
	if parentID != "" {
		if err := t.computeAndStoreDiff(ctx, tx, parentID, versionID); err != nil {
			t.logger.Warn("Failed to compute diff, continuing", interfaces.Field{Key: "error", Value: err.Error()})
			// Don't fail the commit if diff computation fails
		}
	}

	// Step 9: Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Step 10: Update working tree
	// TODO: Implement working tree update with AtomicWriteFile
	// For now, skip updating the working tree

	// Step 11: Write HEAD
	if err := t.writeHEAD(versionID); err != nil {
		t.logger.Warn("Failed to update HEAD", interfaces.Field{Key: "error", Value: err.Error()})
		// Don't fail the commit if HEAD update fails
	}

	t.logger.Info("Commit successful", 
		interfaces.Field{Key: "versionID", Value: versionID},
		interfaces.Field{Key: "snapshotID", Value: snapshotID})

	// Return the created version
	return &model.Version{
		ID:         versionID,
		Parent:     parentID,
		Message:    message,
		Author:     author,
		SnapshotID: snapshotID,
		Timestamp:  time.Unix(timestamp, 0),
	}, nil
}

// Diff computes a delta between two versions identified by their IDs.
func (t *SQLiteTracker) Diff(ctx context.Context, baseID, headID string) (*model.DiffResult, error) {
	t.logger.Debug("Computing diff", 
		interfaces.Field{Key: "baseID", Value: baseID},
		interfaces.Field{Key: "headID", Value: headID})

	// Check if diff already exists in cache
	var diffJSON string
	err := t.db.QueryRowContext(ctx, `
		SELECT diff_json FROM diffs
		WHERE base_version_id = ? AND head_version_id = ?
	`, nullableString(baseID), headID).Scan(&diffJSON)

	if err == nil {
		// Diff exists, parse and return
		var result model.DiffResult
		if err := json.Unmarshal([]byte(diffJSON), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached diff: %w", err)
		}
		return &result, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query cached diff: %w", err)
	}

	// Diff not cached, compute it
	// TODO: Implement actual diff computation
	// For now, return a placeholder
	return &model.DiffResult{
		BaseID: baseID,
		HeadID: headID,
		Chunks: []model.DiffChunk{
			{
				Type:    "modified",
				Path:    "",
				Content: "Diff computation not yet fully implemented",
			},
		},
	}, nil
}

// Get returns the snapshot for a specific version ID.
func (t *SQLiteTracker) Get(ctx context.Context, versionID string) (*model.Snapshot, error) {
	t.logger.Debug("Getting snapshot", interfaces.Field{Key: "versionID", Value: versionID})

	// Query snapshot ID from version
	var snapshotID, url, filePath string
	var createdAt int64
	err := t.db.QueryRowContext(ctx, `
		SELECT s.id, s.url, s.file_path, s.created_at
		FROM snapshots s
		JOIN versions v ON s.id = v.snapshot_id
		WHERE v.id = ?
	`, versionID).Scan(&snapshotID, &url, &filePath, &createdAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version not found: %s", versionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	// Get blob ID for the file
	var blobID string
	err = t.db.QueryRowContext(ctx, `
		SELECT blob_id FROM version_files
		WHERE version_id = ? AND file_path = ?
	`, versionID, filePath).Scan(&blobID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file not found in version: %s", filePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query version_files: %w", err)
	}

	// Get blob content
	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}

	// TODO: Reconstruct meta from stored metadata
	return &model.Snapshot{
		ID:        snapshotID,
		URL:       url,
		Body:      body,
		Meta:      make(map[string]string),
		CreatedAt: time.Unix(createdAt, 0),
	}, nil
}

// List returns recent versions (head-first).
func (t *SQLiteTracker) List(ctx context.Context, limit int) ([]*model.Version, error) {
	t.logger.Debug("Listing versions", interfaces.Field{Key: "limit", Value: limit})

	if limit <= 0 {
		limit = 10 // Default limit
	}

	rows, err := t.db.QueryContext(ctx, `
		SELECT id, parent_id, snapshot_id, message, author, timestamp
		FROM versions
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	var versions []*model.Version
	for rows.Next() {
		var v model.Version
		var parentID, author sql.NullString
		var timestamp int64

		if err := rows.Scan(&v.ID, &parentID, &v.SnapshotID, &v.Message, &author, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}

		if parentID.Valid {
			v.Parent = parentID.String
		}
		if author.Valid {
			v.Author = author.String
		}
		v.Timestamp = time.Unix(timestamp, 0)

		versions = append(versions, &v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating versions: %w", err)
	}

	return versions, nil
}

// Checkout updates the working tree to match a specific version.
// TODO: Implement full checkout logic
func (t *SQLiteTracker) Checkout(ctx context.Context, versionID string) error {
	t.logger.Debug("Checkout version", interfaces.Field{Key: "versionID", Value: versionID})
	return ErrNotImplemented
}

// Close releases resources used by the tracker.
func (t *SQLiteTracker) Close() error {
	t.logger.Info("Closing SQLiteTracker")
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

// Helper methods

// computeAndStoreDiff computes a diff between base and head versions and stores it.
func (t *SQLiteTracker) computeAndStoreDiff(ctx context.Context, tx *sql.Tx, baseID, headID string) error {
	// Get snapshot bodies for both versions
	baseBody, err := t.getVersionBody(ctx, tx, baseID)
	if err != nil {
		return fmt.Errorf("failed to get base version body: %w", err)
	}

	headBody, err := t.getVersionBody(ctx, tx, headID)
	if err != nil {
		return fmt.Errorf("failed to get head version body: %w", err)
	}

	// Compute diff (placeholder)
	diffJSON, err := computeTextDiffJSON(baseID, headID, baseBody, headBody)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	// Store diff
	diffID := uuid.New().String()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())

	return err
}

// getVersionBody retrieves the body content for a version.
func (t *SQLiteTracker) getVersionBody(ctx context.Context, tx *sql.Tx, versionID string) ([]byte, error) {
	// Get blob ID from version_files (assuming single file for now)
	var blobID string
	err := tx.QueryRowContext(ctx, `
		SELECT blob_id FROM version_files
		WHERE version_id = ?
		LIMIT 1
	`, versionID).Scan(&blobID)

	if err != nil {
		return nil, err
	}

	// Get blob content
	return t.store.Get(blobID)
}

// readHEAD reads the current HEAD version ID.
func (t *SQLiteTracker) readHEAD() (string, error) {
	headPath := filepath.Join(t.siteDir, ".moku", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeHEAD writes the current HEAD version ID.
func (t *SQLiteTracker) writeHEAD(versionID string) error {
	headPath := filepath.Join(t.siteDir, ".moku", "HEAD")
	return AtomicWriteFile(headPath, []byte(versionID), 0644)
}

// nullableString converts an empty string to sql.NullString.
func nullableString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}
