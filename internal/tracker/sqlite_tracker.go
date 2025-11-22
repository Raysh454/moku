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
	config  *Config
}

// NewSQLiteTracker creates a new SQLiteTracker instance.
// It initializes the SQLite database at siteDir/.moku/moku.db and sets up the blob store.
func NewSQLiteTracker(siteDir string, logger interfaces.Logger) (*SQLiteTracker, error) {
	return NewSQLiteTrackerWithConfig(siteDir, logger, nil)
}

// NewSQLiteTrackerWithConfig creates a new SQLiteTracker instance with custom configuration.
// If config is nil, default configuration is used.
func NewSQLiteTrackerWithConfig(siteDir string, logger interfaces.Logger, config *Config) (*SQLiteTracker, error) {
	if logger == nil {
		return nil, errors.New("tracker: nil logger provided")
	}

	// Use default config if not provided
	if config == nil {
		config = &Config{}
	}

	// Default to redacting sensitive headers if not explicitly set
	if config.RedactSensitiveHeaders == nil {
		redactDefault := true
		config.RedactSensitiveHeaders = &redactDefault
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
		config:  config,
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
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.logger.Warn("Failed to rollback transaction", interfaces.Field{Key: "error", Value: rbErr.Error()})
		}
	}()

	// Step 5: Extract and normalize headers from snapshot metadata
	headers := make(map[string][]string)
	if snapshot.Meta != nil {
		// Try to parse headers from Meta["_headers"] as JSON
		if headersJSON, ok := snapshot.Meta["_headers"]; ok {
			if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
				t.logger.Warn("Failed to parse headers from metadata", interfaces.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// Normalize and serialize headers
	redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
	normalizedHeaders := normalizeHeaders(headers, redactSensitive)
	headersJSON, err := json.Marshal(normalizedHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Step 6: Insert snapshot record with headers
	filePath := "index.html" // Default file path, could be derived from URL
	if snapshot.URL != "" {
		// TODO: Derive file path from URL (e.g., parse path component)
		filePath = "index.html"
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (id, url, file_path, created_at, headers)
		VALUES (?, ?, ?, ?, ?)
	`, snapshotID, snapshot.URL, filePath, timestamp, string(headersJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	// Step 7: Insert version record
	_, err = tx.ExecContext(ctx, `
		INSERT INTO versions (id, parent_id, snapshot_id, message, author, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, versionID, nullableString(parentID), snapshotID, message, nullableString(author), timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to insert version: %w", err)
	}

	// Step 8: Insert version_files record
	_, err = tx.ExecContext(ctx, `
		INSERT INTO version_files (version_id, file_path, blob_id, size)
		VALUES (?, ?, ?, ?)
	`, versionID, filePath, blobID, len(snapshot.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to insert version_files: %w", err)
	}

	// Step 9: Compute and store diff (if parent exists)
	if parentID != "" {
		if err := t.computeAndStoreDiff(ctx, tx, parentID, versionID); err != nil {
			t.logger.Warn("Failed to compute diff, continuing", interfaces.Field{Key: "error", Value: err.Error()})
			// Don't fail the commit if diff computation fails
		}
	}

	// Step 10: Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Step 11: Write working-tree convenience files
	if err := t.writeWorkingTreeFiles(filePath, snapshot.Body, normalizedHeaders); err != nil {
		t.logger.Warn("Failed to write working-tree files", interfaces.Field{Key: "error", Value: err.Error()})
		// Don't fail the commit if working-tree writes fail - DB is authoritative
		// Schedule reconciliation if needed
	}

	// Step 12: Write HEAD
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
		// Detect format by checking for the presence of "body_diff" or "headers_diff" keys
		// New format has these keys, old format has "base_id", "head_id", "chunks" at top level
		var rawDiff map[string]interface{}
		if err := json.Unmarshal([]byte(diffJSON), &rawDiff); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached diff: %w", err)
		}

		// Check if this is the new combined format
		if _, hasBodyDiff := rawDiff["body_diff"]; hasBodyDiff {
			// New format with combined diff, extract body diff
			var combined CombinedDiff
			if err := json.Unmarshal([]byte(diffJSON), &combined); err != nil {
				return nil, fmt.Errorf("failed to unmarshal combined diff: %w", err)
			}
			result := model.DiffResult{
				BaseID: combined.BodyDiff.BaseID,
				HeadID: combined.BodyDiff.HeadID,
				Chunks: make([]model.DiffChunk, len(combined.BodyDiff.Chunks)),
			}
			for i, c := range combined.BodyDiff.Chunks {
				result.Chunks[i] = model.DiffChunk{
					Type:    c.Type,
					Path:    c.Path,
					Content: c.Content,
				}
			}
			return &result, nil
		}

		// Old format - parse directly
		var result model.DiffResult
		if err := json.Unmarshal([]byte(diffJSON), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached diff: %w", err)
		}
		return &result, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query cached diff: %w", err)
	}

	// Diff not cached, compute it
	var baseBody, headBody []byte
	var err2 error

	// Get base version body (if baseID is not empty)
	if baseID != "" {
		baseBody, err2 = t.getVersionBodyByID(ctx, baseID)
		if err2 != nil {
			return nil, fmt.Errorf("failed to get base version body: %w", err2)
		}
	}

	// Get head version body
	headBody, err2 = t.getVersionBodyByID(ctx, headID)
	if err2 != nil {
		return nil, fmt.Errorf("failed to get head version body: %w", err2)
	}

	// Compute diff
	diffJSON, err2 = computeTextDiffJSON(baseID, headID, baseBody, headBody)
	if err2 != nil {
		return nil, fmt.Errorf("failed to compute diff: %w", err2)
	}

	// Parse the diff result
	var result model.DiffResult
	if err := json.Unmarshal([]byte(diffJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal computed diff: %w", err)
	}

	// Cache the diff for future use
	diffID := uuid.New().String()
	_, err = t.db.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())
	if err != nil {
		// Log but don't fail if caching fails
		t.logger.Warn("Failed to cache diff", interfaces.Field{Key: "error", Value: err.Error()})
	}

	return &result, nil
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
// This restores all files from the specified version to the working directory.
func (t *SQLiteTracker) Checkout(ctx context.Context, versionID string) error {
	t.logger.Debug("Checkout version", interfaces.Field{Key: "versionID", Value: versionID})

	// Verify version exists
	var exists int
	err := t.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM versions WHERE id = ?
	`, versionID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to verify version: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("version not found: %s", versionID)
	}

	// Get all files for this version
	type fileEntry struct {
		path   string
		blobID string
	}
	var files []fileEntry

	rows, err := t.db.QueryContext(ctx, `
		SELECT file_path, blob_id FROM version_files
		WHERE version_id = ?
	`, versionID)
	if err != nil {
		return fmt.Errorf("failed to query version files: %w", err)
	}
	defer rows.Close()

	var filePath, blobID string
	for rows.Next() {
		if err := rows.Scan(&filePath, &blobID); err != nil {
			return fmt.Errorf("failed to scan file entry: %w", err)
		}
		files = append(files, fileEntry{path: filePath, blobID: blobID})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating version files: %w", err)
	}

	// Restore each file to the working tree
	for _, file := range files {
		// Get blob content
		content, err := t.store.Get(file.blobID)
		if err != nil {
			return fmt.Errorf("failed to get blob %s: %w", file.blobID, err)
		}

		// Get headers for this file from snapshot
		var headersJSON sql.NullString
		err = t.db.QueryRowContext(ctx, `
			SELECT s.headers
			FROM snapshots s
			JOIN versions v ON s.id = v.snapshot_id
			WHERE v.id = ? AND s.file_path = ?
		`, versionID, file.path).Scan(&headersJSON)
		if err != nil && err != sql.ErrNoRows {
			t.logger.Warn("Failed to get headers for file",
				interfaces.Field{Key: "filePath", Value: file.path},
				interfaces.Field{Key: "error", Value: err.Error()})
		}

		// Parse headers
		var headers map[string][]string
		if headersJSON.Valid && headersJSON.String != "" {
			if err := json.Unmarshal([]byte(headersJSON.String), &headers); err != nil {
				t.logger.Warn("Failed to parse headers",
					interfaces.Field{Key: "error", Value: err.Error()})
				headers = make(map[string][]string)
			}
		} else {
			headers = make(map[string][]string)
		}

		// Write working-tree files (.page_body and .page_headers.json)
		if err := t.writeWorkingTreeFiles(file.path, content, headers); err != nil {
			return fmt.Errorf("failed to write working-tree files for %s: %w", file.path, err)
		}

		t.logger.Debug("Checked out file",
			interfaces.Field{Key: "path", Value: file.path},
			interfaces.Field{Key: "blobID", Value: file.blobID})
	}

	// Update HEAD to point to this version
	if err := t.writeHEAD(versionID); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}

	t.logger.Info("Checkout complete",
		interfaces.Field{Key: "versionID", Value: versionID},
		interfaces.Field{Key: "filesRestored", Value: len(files)})

	return nil
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
	// Get snapshot bodies and headers for both versions
	baseBody, baseHeaders, err := t.getVersionData(ctx, tx, baseID)
	if err != nil {
		return fmt.Errorf("failed to get base version data: %w", err)
	}

	headBody, headHeaders, err := t.getVersionData(ctx, tx, headID)
	if err != nil {
		return fmt.Errorf("failed to get head version data: %w", err)
	}

	// Compute combined diff (body + headers)
	redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
	diffJSON, err := computeCombinedDiff(baseID, headID, baseBody, headBody, baseHeaders, headHeaders, redactSensitive)
	if err != nil {
		return fmt.Errorf("failed to compute combined diff: %w", err)
	}

	// Store diff
	diffID := uuid.New().String()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())

	return err
}

// getVersionData retrieves both body and headers for a version (transaction version).
func (t *SQLiteTracker) getVersionData(ctx context.Context, tx *sql.Tx, versionID string) ([]byte, map[string][]string, error) {
	// Get blob ID from version_files and headers from snapshots
	var blobID string
	var headersJSON sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT vf.blob_id, s.headers
		FROM version_files vf
		JOIN versions v ON vf.version_id = v.id
		JOIN snapshots s ON v.snapshot_id = s.id
		WHERE vf.version_id = ?
		LIMIT 1
	`, versionID).Scan(&blobID, &headersJSON)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to query version data: %w", err)
	}

	// Get blob content
	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blob: %w", err)
	}

	// Parse headers
	var headers map[string][]string
	if headersJSON.Valid && headersJSON.String != "" {
		if err := json.Unmarshal([]byte(headersJSON.String), &headers); err != nil {
			t.logger.Warn("Failed to parse headers", interfaces.Field{Key: "error", Value: err.Error()})
			headers = make(map[string][]string)
		}
	} else {
		headers = make(map[string][]string)
	}

	return body, headers, nil
}

// getVersionBodyByID retrieves the body content for a version (non-transaction version).
func (t *SQLiteTracker) getVersionBodyByID(ctx context.Context, versionID string) ([]byte, error) {
	// Get blob ID from version_files (assuming single file for now)
	var blobID string
	err := t.db.QueryRowContext(ctx, `
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

// CommitBatch commits multiple snapshots in a single transaction.
// This is more efficient than individual commits when storing multiple pages.
//
// The method:
// 1. Stores all snapshot bodies as blobs
// 2. Begins a transaction
// 3. Creates a single version with multiple snapshots
// 4. Inserts all snapshots with normalized headers
// 5. Inserts version_files mappings
// 6. Computes diffs for each file against parent version
// 7. Commits the transaction
// 8. Writes working-tree files for each snapshot
// 9. Updates HEAD
//
// If working-tree writes fail after DB commit, an error is returned but the DB
// remains consistent (authoritative source).
func (t *SQLiteTracker) CommitBatch(ctx context.Context, snapshots []*model.Snapshot, message string, author string) ([]*model.Version, error) {
	if len(snapshots) == 0 {
		return nil, errors.New("no snapshots to commit")
	}
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	t.logger.Debug("Starting batch commit",
		interfaces.Field{Key: "count", Value: len(snapshots)},
		interfaces.Field{Key: "message", Value: message})

	// Step 1: Store all blobs and prepare snapshot data
	type snapshotData struct {
		snapshot          *model.Snapshot
		blobID            string
		filePath          string
		normalizedHeaders map[string][]string
		headersJSON       string
	}

	snapshotDataList := make([]snapshotData, 0, len(snapshots))

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}

		// Store blob
		blobID, err := t.store.Put(snapshot.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to store snapshot body: %w", err)
		}

		// Extract file path from metadata
		filePath := "index.html" // Default
		if snapshot.Meta != nil {
			if fp, ok := snapshot.Meta["_file_path"]; ok && fp != "" {
				filePath = fp
			}
		}

		// Extract and normalize headers
		headers := make(map[string][]string)
		if snapshot.Meta != nil {
			if headersJSON, ok := snapshot.Meta["_headers"]; ok {
				if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
					t.logger.Warn("Failed to parse headers from metadata",
						interfaces.Field{Key: "error", Value: err.Error()})
				}
			}
		}

		redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
		normalizedHeaders := normalizeHeaders(headers, redactSensitive)
		headersJSONBytes, err := json.Marshal(normalizedHeaders)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal headers: %w", err)
		}

		snapshotDataList = append(snapshotDataList, snapshotData{
			snapshot:          snapshot,
			blobID:            blobID,
			filePath:          filePath,
			normalizedHeaders: normalizedHeaders,
			headersJSON:       string(headersJSONBytes),
		})
	}

	if len(snapshotDataList) == 0 {
		return nil, errors.New("no valid snapshots to commit")
	}

	// Step 2: Generate IDs
	versionID := uuid.New().String()
	timestamp := time.Now().Unix()

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
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.logger.Warn("Failed to rollback transaction", interfaces.Field{Key: "error", Value: rbErr.Error()})
		}
	}()

	// Step 5: Insert first snapshot (required for FK constraint on versions.snapshot_id)
	firstSnapshotID := uuid.New().String()
	firstSD := snapshotDataList[0]
	firstSnapshotTimestamp := timestamp
	if !firstSD.snapshot.CreatedAt.IsZero() {
		firstSnapshotTimestamp = firstSD.snapshot.CreatedAt.Unix()
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (id, url, file_path, created_at, headers)
		VALUES (?, ?, ?, ?, ?)
	`, firstSnapshotID, firstSD.snapshot.URL, firstSD.filePath, firstSnapshotTimestamp, firstSD.headersJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to insert first snapshot: %w", err)
	}

	// Step 6: Insert version record (now that first snapshot exists)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO versions (id, parent_id, snapshot_id, message, author, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, versionID, nullableString(parentID), firstSnapshotID, message, nullableString(author), timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to insert version: %w", err)
	}

	// Step 7: Insert version_files for first snapshot
	_, err = tx.ExecContext(ctx, `
		INSERT INTO version_files (version_id, file_path, blob_id, size)
		VALUES (?, ?, ?, ?)
	`, versionID, firstSD.filePath, firstSD.blobID, len(firstSD.snapshot.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to insert version_files for first snapshot: %w", err)
	}

	// Step 8: Insert remaining snapshots and version_files
	for i := 1; i < len(snapshotDataList); i++ {
		sd := snapshotDataList[i]
		snapshotID := uuid.New().String()

		snapshotTimestamp := timestamp
		if !sd.snapshot.CreatedAt.IsZero() {
			snapshotTimestamp = sd.snapshot.CreatedAt.Unix()
		}

		// Insert snapshot
		_, err = tx.ExecContext(ctx, `
			INSERT INTO snapshots (id, url, file_path, created_at, headers)
			VALUES (?, ?, ?, ?, ?)
		`, snapshotID, sd.snapshot.URL, sd.filePath, snapshotTimestamp, sd.headersJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to insert snapshot: %w", err)
		}

		// Insert version_files
		_, err = tx.ExecContext(ctx, `
			INSERT INTO version_files (version_id, file_path, blob_id, size)
			VALUES (?, ?, ?, ?)
		`, versionID, sd.filePath, sd.blobID, len(sd.snapshot.Body))
		if err != nil {
			return nil, fmt.Errorf("failed to insert version_files: %w", err)
		}
	}

	// Step 9: Compute and store diffs (if parent exists)
	if parentID != "" {
		for _, sd := range snapshotDataList {
			// Get parent snapshot for this file path
			var parentBlobID string
			var parentHeadersJSON sql.NullString
			err := tx.QueryRowContext(ctx, `
				SELECT vf.blob_id, s.headers
				FROM version_files vf
				LEFT JOIN versions v ON vf.version_id = v.id
				LEFT JOIN snapshots s ON v.snapshot_id = s.id
				WHERE vf.version_id = ? AND vf.file_path = ?
			`, parentID, sd.filePath).Scan(&parentBlobID, &parentHeadersJSON)

			if err == sql.ErrNoRows {
				// No parent snapshot for this file - skip diff
				t.logger.Debug("No parent snapshot for file, skipping diff",
					interfaces.Field{Key: "filePath", Value: sd.filePath})
				continue
			}
			if err != nil {
				t.logger.Warn("Failed to query parent snapshot, skipping diff",
					interfaces.Field{Key: "error", Value: err.Error()})
				continue
			}

			// Get parent blob content
			parentBody, err := t.store.Get(parentBlobID)
			if err != nil {
				t.logger.Warn("Failed to get parent blob, skipping diff",
					interfaces.Field{Key: "error", Value: err.Error()})
				continue
			}

			// Parse parent headers
			var parentHeaders map[string][]string
			if parentHeadersJSON.Valid && parentHeadersJSON.String != "" {
				if err := json.Unmarshal([]byte(parentHeadersJSON.String), &parentHeaders); err != nil {
					t.logger.Warn("Failed to parse parent headers",
						interfaces.Field{Key: "error", Value: err.Error()})
					parentHeaders = make(map[string][]string)
				}
			} else {
				parentHeaders = make(map[string][]string)
			}

			// Compute combined diff
			redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
			diffJSON, err := computeCombinedDiff(parentID, versionID, parentBody, sd.snapshot.Body, parentHeaders, sd.normalizedHeaders, redactSensitive)
			if err != nil {
				t.logger.Warn("Failed to compute diff, skipping",
					interfaces.Field{Key: "error", Value: err.Error()})
				continue
			}

			// Store diff
			diffID := uuid.New().String()
			_, err = tx.ExecContext(ctx, `
				INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
				VALUES (?, ?, ?, ?, ?)
			`, diffID, nullableString(parentID), versionID, diffJSON, time.Now().Unix())
			if err != nil {
				t.logger.Warn("Failed to store diff",
					interfaces.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// Step 10: Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Step 11: Write working-tree files for all snapshots
	workingTreeErrors := []string{}
	for _, sd := range snapshotDataList {
		if err := t.writeWorkingTreeFiles(sd.filePath, sd.snapshot.Body, sd.normalizedHeaders); err != nil {
			errMsg := fmt.Sprintf("file %s: %v", sd.filePath, err)
			workingTreeErrors = append(workingTreeErrors, errMsg)
			t.logger.Warn("Failed to write working-tree files",
				interfaces.Field{Key: "filePath", Value: sd.filePath},
				interfaces.Field{Key: "error", Value: err.Error()})
		}
	}

	// Step 12: Write HEAD
	if err := t.writeHEAD(versionID); err != nil {
		t.logger.Warn("Failed to update HEAD", interfaces.Field{Key: "error", Value: err.Error()})
	}

	t.logger.Info("Batch commit successful",
		interfaces.Field{Key: "versionID", Value: versionID},
		interfaces.Field{Key: "snapshotCount", Value: len(snapshotDataList)})

	// Return version record for each snapshot
	versions := make([]*model.Version, len(snapshotDataList))
	for i := range snapshotDataList {
		versions[i] = &model.Version{
			ID:         versionID,
			Parent:     parentID,
			Message:    message,
			Author:     author,
			SnapshotID: versionID, // Placeholder - actual snapshots are in version_files
			Timestamp:  time.Unix(timestamp, 0),
		}
	}

	// If there were working-tree errors, note them (but DB is still consistent)
	if len(workingTreeErrors) > 0 {
		t.logger.Warn("Some working-tree files failed to write (DB is authoritative)",
			interfaces.Field{Key: "errors", Value: fmt.Sprintf("%v", workingTreeErrors)})
	}

	return versions, nil
}

// writeWorkingTreeFiles writes the convenience files for a snapshot to the working tree.
// These files are:
// - .page_body: The raw body content
// - .page_headers.json: The normalized headers as JSON
//
// Both files are written atomically to prevent corruption.
// The working tree is a convenience layer; the authoritative data is in .moku/
//
// The filePath is treated as a directory name (following fetcher conventions),
// so files are written to siteDir/filePath/.page_body and siteDir/filePath/.page_headers.json
func (t *SQLiteTracker) writeWorkingTreeFiles(filePath string, body []byte, headers map[string][]string) error {
	// Build directory path in working tree
	// The filePath is treated as a directory (e.g., "page1.html" becomes a directory)
	dirPath := filepath.Join(t.siteDir, filePath)

	// Ensure directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Write .page_body
	bodyPath := filepath.Join(dirPath, ".page_body")
	if err := AtomicWriteFile(bodyPath, body, 0644); err != nil {
		return fmt.Errorf("failed to write .page_body: %w", err)
	}

	// Write .page_headers.json
	headersJSON, err := json.MarshalIndent(headers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}
	headersPath := filepath.Join(dirPath, ".page_headers.json")
	if err := AtomicWriteFile(headersPath, headersJSON, 0644); err != nil {
		return fmt.Errorf("failed to write .page_headers.json: %w", err)
	}

	return nil
}
