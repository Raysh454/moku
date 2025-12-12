package tracker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
	_ "modernc.org/sqlite" // SQLite driver
)

var (
	ErrProjectIDEmpty    = errors.New("project id is empty")
	ErrProjectIDMismatch = errors.New("project id mismatch")
)

// SQLiteTracker implements interfaces.Tracker using SQLite for metadata storage
// and a content-addressed blob store for file content.
type SQLiteTracker struct {
	db       *sql.DB
	store    *FSStore
	logger   logging.Logger
	config   *Config
	assessor assessor.Assessor
}

// NewSQLiteTracker creates a new SQLiteTracker instance with custom configuration.
// If config is nil, default configuration is used.
func NewSQLiteTracker(logger logging.Logger, config *Config) (*SQLiteTracker, error) {
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
	mokuDir := filepath.Join(config.StoragePath, ".moku")
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

	logger.Info("SQLiteTracker initialized", logging.Field{Key: "config.StoragePath", Value: config.StoragePath})

	t := &SQLiteTracker{
		db:     db,
		store:  store,
		logger: logger,
		config: config,
	}

	if config.ProjectID != "" {
		if err := t.SetProjectID(context.Background(), config.ProjectID, config.ForceProjectID); err != nil {
			// prefer failing fast so mismatch doesn't go unnoticed:
			db.Close()
			return nil, fmt.Errorf("project id mismatch or set failed: %w", err)
		}
	} else {
		// optionally read and log
		if pid, _ := t.GetProjectID(context.Background()); pid == "" {
			t.logger.Info("no project_id set in DB meta; set via t.SetProjectID when available")
		} else {
			t.logger.Info("project_id loaded from DB meta", logging.Field{Key: "project_id", Value: pid})
		}
	}

	return t, nil
}

// SetAssessor sets the assessor implementation the tracker should use when scoring.
func (t *SQLiteTracker) SetAssessor(a assessor.Assessor) {
	t.assessor = a
}

// GetProjectID returns the project_id from meta or sql.ErrNoRows if not present.
func (t *SQLiteTracker) GetProjectID(ctx context.Context) (string, error) {
	var v sql.NullString
	if err := t.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, "project_id").Scan(&v); err != nil {
		return "", err
	}
	if !v.Valid {
		return "", nil
	}
	return v.String, nil
}

// SetProjectID sets project_id in meta.
// If force==false and an existing value differs, returns ErrProjectIDMismatch.
// The operation is atomic via a short transaction.
func (t *SQLiteTracker) SetProjectID(ctx context.Context, projectID string, force bool) error {
	if projectID == "" {
		return ErrProjectIDEmpty
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var existing sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, "project_id").Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("query meta: %w", err)
	}

	if err == sql.ErrNoRows || !existing.Valid {
		// insert
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?)`, "project_id", projectID); err != nil {
			return fmt.Errorf("insert meta: %w", err)
		}
	} else {
		// existing present
		if existing.String == projectID {
			// idempotent no-op
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit: %w", err)
			}
			return nil
		}
		if !force {
			return ErrProjectIDMismatch
		}
		// overwrite intentionally
		if _, err := tx.ExecContext(ctx, `UPDATE meta SET value = ? WHERE key = ?`, projectID, "project_id"); err != nil {
			return fmt.Errorf("update meta: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Ensure SQLiteTracker implements interfaces.Tracker at compile-time.
var _ Tracker = (*SQLiteTracker)(nil)

// Commit stores a snapshot and returns a CommitResult.
func (t *SQLiteTracker) Commit(ctx context.Context, snapshot *Snapshot, message string, author string) (*CommitResult, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot cannot be nil")
	}
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	t.logger.Debug("Starting commit",
		logging.Field{Key: "url", Value: snapshot.URL},
		logging.Field{Key: "message", Value: message})

	blobID, err := t.store.Put(snapshot.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to store snapshot body: %w", err)
	}
	t.logger.Debug("Stored snapshot body", logging.Field{Key: "blobID", Value: blobID})

	snapshotID := uuid.New().String()
	versionID := uuid.New().String()
	timestamp := time.Now().Unix()
	if !snapshot.CreatedAt.IsZero() {
		timestamp = snapshot.CreatedAt.Unix()
	}

	parentID, err := t.readHEAD()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read HEAD: %w", err)
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.logger.Warn("Failed to rollback transaction", logging.Field{Key: "error", Value: rbErr.Error()})
		}
	}()

	redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
	normalizedHeaders := normalizeHeaders(snapshot.Headers, redactSensitive)
	headersJSON, err := json.Marshal(normalizedHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	urlTools, err := utils.NewURLTools(snapshot.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse snapshot URL: %w", err)
	}
	filePath := urlTools.GetPath()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (id, status_code, url, file_path, created_at, headers)
		VALUES (?, ?, ?, ?, ?, ?)
	`, snapshotID, snapshot.StatusCode, snapshot.URL, filePath, timestamp, string(headersJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO versions (id, parent_id, snapshot_id, message, author, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, versionID, nullableString(parentID), snapshotID, message, nullableString(author), timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to insert version: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO version_files (version_id, file_path, blob_id, size)
		VALUES (?, ?, ?, ?)
	`, versionID, filePath, blobID, len(snapshot.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to insert version_files: %w", err)
	}

	if parentID != "" {
		if err := t.computeAndStoreDiff(ctx, tx, parentID, versionID); err != nil {
			t.logger.Warn("Failed to compute diff, continuing", logging.Field{Key: "error", Value: err.Error()})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if err := t.writeWorkingTreeFiles(filePath, snapshot.StatusCode, snapshot.Body, normalizedHeaders); err != nil {
		t.logger.Warn("Failed to write working-tree files", logging.Field{Key: "error", Value: err.Error()})
	}

	if err := t.writeHEAD(versionID); err != nil {
		t.logger.Warn("Failed to update HEAD", logging.Field{Key: "error", Value: err.Error()})
	}

	t.logger.Info("Commit successful",
		logging.Field{Key: "versionID", Value: versionID},
		logging.Field{Key: "snapshotID", Value: snapshotID})

	var diffID, diffJSON string
	if err := t.db.QueryRowContext(ctx, `SELECT id, diff_json FROM diffs WHERE head_version_id = ? ORDER BY created_at DESC LIMIT 1`, versionID).Scan(&diffID, &diffJSON); err != nil && err != sql.ErrNoRows {
		t.logger.Warn("failed to fetch diff row after commit", logging.Field{Key: "err", Value: err})
	}

	return &CommitResult{
		Version:         Version{ID: versionID, Parent: parentID, Message: message, Author: author, SnapshotID: snapshotID, Timestamp: time.Unix(timestamp, 0)},
		ParentVersionID: parentID,
		DiffID:          diffID,
		DiffJSON:        diffJSON,
		HeadBody:        snapshot.Body,
		HeadBlobID:      blobID,
		HeadFilePath:    filePath,
		Opts:            assessor.ScoreOptions{RequestLocations: true},
	}, nil
}

func (t *SQLiteTracker) diffFromCache(diffJSON string) (*DiffResult, error) {
	var combined CombinedDiff
	if err := json.Unmarshal([]byte(diffJSON), &combined); err != nil {
		return nil, fmt.Errorf("failed to unmarshal combined diff: %w", err)
	}
	result := DiffResult{
		BaseID: combined.BodyDiff.BaseID,
		HeadID: combined.BodyDiff.HeadID,
		Chunks: make([]DiffChunk, len(combined.BodyDiff.Chunks)),
	}
	for i, c := range combined.BodyDiff.Chunks {
		result.Chunks[i] = DiffChunk(c)
	}
	return &result, nil
}

func (t *SQLiteTracker) computeDiff(ctx context.Context, baseID, headID string) (*DiffResult, string, error) {
	var baseBody, headBody []byte
	var err error

	if baseID != "" {
		baseBody, err = t.getVersionBodyByID(ctx, baseID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get base version body: %w", err)
		}
	}

	headBody, err = t.getVersionBodyByID(ctx, headID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get head version body: %w", err)
	}

	diffJSON, err := computeTextDiffJSON(baseID, headID, baseBody, headBody)
	if err != nil {
		return nil, "", fmt.Errorf("failed to compute diff: %w", err)
	}

	var result DiffResult
	if err := json.Unmarshal([]byte(diffJSON), &result); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal computed diff: %w", err)
	}

	return &result, diffJSON, nil
}

// Diff computes a delta between two versions identified by their IDs.
func (t *SQLiteTracker) Diff(ctx context.Context, baseID, headID string) (*DiffResult, error) {
	t.logger.Debug("Computing diff",
		logging.Field{Key: "baseID", Value: baseID},
		logging.Field{Key: "headID", Value: headID})

	var diffJSON string
	err := t.db.QueryRowContext(ctx, `
		SELECT diff_json FROM diffs
		WHERE base_version_id = ? AND head_version_id = ?
	`, nullableString(baseID), headID).Scan(&diffJSON)

	if err == nil {
		return t.diffFromCache(diffJSON)
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query cached diff: %w", err)
	}

	result, diffJSON, err := t.computeDiff(ctx, baseID, headID)
	if err != nil {
		return nil, err
	}

	diffID := uuid.New().String()
	_, err = t.db.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())
	if err != nil {
		t.logger.Warn("Failed to cache diff", logging.Field{Key: "error", Value: err.Error()})
	}

	return result, nil
}

// Get returns the snapshot for a specific version ID.
// TODO: Multiple snapshots can have the same version, so return all of them.
func (t *SQLiteTracker) Get(ctx context.Context, versionID string) (*Snapshot, error) {
	t.logger.Debug("Getting snapshots", logging.Field{Key: "versionID", Value: versionID})

	var snapshotID, url, filePath string
	var createdAt int64
	var statucode int
	var headersJSONSQL sql.NullString
	err := t.db.QueryRowContext(ctx, `
		SELECT s.id, s.status_code, s.url, s.file_path, s.created_at, s.headers
		FROM snapshots s
		JOIN versions v ON s.id = v.snapshot_id
		WHERE v.id = ?
	`, versionID).Scan(&snapshotID, &statucode, &url, &filePath, &createdAt, &headersJSONSQL)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version not found: %s", versionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

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

	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}

	headersJSON := headersJSONSQL.String
	var headers map[string][]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		t.logger.Warn("Failed to parse headers", logging.Field{Key: "error", Value: err.Error()})
	}

	return &Snapshot{
		ID:         snapshotID,
		StatusCode: statucode,
		URL:        url,
		Body:       body,
		Headers:    headers,
		CreatedAt:  time.Unix(createdAt, 0),
	}, nil
}

// List returns recent versions (head-first).
func (t *SQLiteTracker) List(ctx context.Context, limit int) ([]*Version, error) {
	t.logger.Debug("Listing versions", logging.Field{Key: "limit", Value: limit})

	if limit <= 0 {
		limit = 10
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

	var versions []*Version
	for rows.Next() {
		var v Version
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
func (t *SQLiteTracker) Checkout(ctx context.Context, versionID string) error {
	t.logger.Debug("Checkout version", logging.Field{Key: "versionID", Value: versionID})

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

	for _, file := range files {
		content, err := t.store.Get(file.blobID)
		if err != nil {
			return fmt.Errorf("failed to get blob %s: %w", file.blobID, err)
		}

		var headersJSON sql.NullString
		var statusCode int
		err = t.db.QueryRowContext(ctx, `
			SELECT s.headers, s.status_code
			FROM snapshots s
			JOIN versions v ON s.id = v.snapshot_id
			WHERE v.id = ? AND s.file_path = ?
		`, versionID, file.path).Scan(&headersJSON, &statusCode)
		if err != nil && err != sql.ErrNoRows {
			t.logger.Warn("Failed to get headers for file",
				logging.Field{Key: "filePath", Value: file.path},
				logging.Field{Key: "error", Value: err.Error()})
		}

		headers := t.parseHeaders(headersJSON)

		if err := t.writeWorkingTreeFiles(file.path, statusCode, content, headers); err != nil {
			return fmt.Errorf("failed to write working-tree files for %s: %w", file.path, err)
		}

		t.logger.Debug("Checked out file",
			logging.Field{Key: "path", Value: file.path},
			logging.Field{Key: "blobID", Value: file.blobID})
	}

	if err := t.writeHEAD(versionID); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}

	t.logger.Info("Checkout complete",
		logging.Field{Key: "versionID", Value: versionID},
		logging.Field{Key: "filesRestored", Value: len(files)})

	return nil
}

func (t *SQLiteTracker) parseHeaders(headersJSON sql.NullString) map[string][]string {
	if !headersJSON.Valid || headersJSON.String == "" {
		return make(map[string][]string)
	}
	var headers map[string][]string
	if err := json.Unmarshal([]byte(headersJSON.String), &headers); err != nil {
		t.logger.Warn("Failed to parse headers", logging.Field{Key: "error", Value: err.Error()})
		return make(map[string][]string)
	}
	return headers
}

// Close releases resources used by the tracker.
func (t *SQLiteTracker) Close() error {
	t.logger.Info("Closing SQLiteTracker")
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *SQLiteTracker) computeAndStoreDiff(ctx context.Context, tx *sql.Tx, baseID, headID string) error {
	baseBody, baseHeaders, err := t.getVersionData(ctx, tx, baseID)
	if err != nil {
		return fmt.Errorf("failed to get base version data: %w", err)
	}

	headBody, headHeaders, err := t.getVersionData(ctx, tx, headID)
	if err != nil {
		return fmt.Errorf("failed to get head version data: %w", err)
	}

	redactSensitive := t.config.RedactSensitiveHeaders != nil && *t.config.RedactSensitiveHeaders
	diffJSON, err := computeCombinedDiff(baseID, headID, baseBody, headBody, baseHeaders, headHeaders, redactSensitive)
	if err != nil {
		return fmt.Errorf("failed to compute combined diff: %w", err)
	}

	diffID := uuid.New().String()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())

	return err
}

func (t *SQLiteTracker) getVersionData(ctx context.Context, tx *sql.Tx, versionID string) ([]byte, map[string][]string, error) {
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

	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blob: %w", err)
	}

	return body, t.parseHeaders(headersJSON), nil
}

func (t *SQLiteTracker) getVersionBodyByID(ctx context.Context, versionID string) ([]byte, error) {
	var blobID string
	err := t.db.QueryRowContext(ctx, `
		SELECT blob_id FROM version_files
		WHERE version_id = ?
		LIMIT 1
	`, versionID).Scan(&blobID)

	if err != nil {
		return nil, err
	}

	return t.store.Get(blobID)
}

func (t *SQLiteTracker) readHEAD() (string, error) {
	headPath := filepath.Join(t.config.StoragePath, ".moku", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (t *SQLiteTracker) writeHEAD(versionID string) error {
	headPath := filepath.Join(t.config.StoragePath, ".moku", "HEAD")
	return AtomicWriteFile(headPath, []byte(versionID), 0644)
}

func nullableString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

type snapshotData struct {
	snapshot    *Snapshot
	snapshotID  string
	blobID      string
	filePath    string
	headersJSON string
}

func (t *SQLiteTracker) insertSnapshot(ctx context.Context, tx *sql.Tx, sd snapshotData) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO snapshots (id, status_code, url, file_path, created_at, headers)
        VALUES (?, ?, ?, ?, ?, ?)
    `,
		sd.snapshotID,
		sd.snapshot.StatusCode,
		sd.snapshot.URL,
		sd.filePath,
		sd.snapshot.CreatedAt.Unix(),
		sd.headersJSON,
	)
	return err
}

func (t *SQLiteTracker) insertVersion(ctx context.Context, tx *sql.Tx,
	versionID, parentID, firstSnapshotID, message, author string, ts int64) error {

	_, err := tx.ExecContext(ctx, `
        INSERT INTO versions (id, parent_id, snapshot_id, message, author, timestamp)
        VALUES (?, ?, ?, ?, ?, ?)
    `,
		versionID,
		nullableString(parentID),
		firstSnapshotID,
		message,
		nullableString(author),
		ts,
	)
	return err
}

func (t *SQLiteTracker) insertVersionFile(ctx context.Context, tx *sql.Tx,
	versionID, path, blobID string, size int) error {

	_, err := tx.ExecContext(ctx, `
        INSERT INTO version_files (version_id, file_path, blob_id, size)
        VALUES (?, ?, ?, ?)
    `,
		versionID, path, blobID, size,
	)
	return err
}

// CommitBatch commits multiple snapshots in a single transaction.
func (t *SQLiteTracker) CommitBatch(ctx context.Context, snapshots []*Snapshot, message, author string) ([]*CommitResult, error) {
	if len(snapshots) == 0 {
		return nil, errors.New("no snapshots to commit")
	}
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	t.logger.Info("Starting batch commit", logging.Field{Key: "count", Value: len(snapshots)})

	var list []snapshotData
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}

		blobID, err := t.store.Put(snap.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to store snapshot body: %w", err)
		}

		urlTools, err := utils.NewURLTools(snap.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse snapshot URL: %w", err)
		}
		filePath := urlTools.GetPath()

		headersJSONBytes, err := json.Marshal(snap.Headers)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal headers: %w", err)
		}

		list = append(list, snapshotData{
			snapshot:    snap,
			snapshotID:  uuid.New().String(),
			blobID:      blobID,
			filePath:    filePath,
			headersJSON: string(headersJSONBytes),
		})
	}

	if len(list) == 0 {
		return nil, errors.New("no valid snapshots to commit")
	}

	versionID := uuid.New().String()
	ts := time.Now().Unix()

	parentID, _ := t.readHEAD()

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.logger.Warn("Failed to rollback transaction", logging.Field{Key: "error", Value: rbErr.Error()})
		}
	}()

	// First snapshot
	first := list[0]
	if err := t.insertSnapshot(ctx, tx, first); err != nil {
		return nil, fmt.Errorf("insert first snapshot: %w", err)
	}

	if err := t.insertVersion(ctx, tx, versionID, parentID, first.snapshotID, message, author, ts); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if err := t.insertVersionFile(ctx, tx, versionID, first.filePath, first.blobID, len(first.snapshot.Body)); err != nil {
		return nil, fmt.Errorf("insert version file: %w", err)
	}

	// Remaining snapshots
	for _, sd := range list[1:] {
		if err := t.insertSnapshot(ctx, tx, sd); err != nil {
			return nil, fmt.Errorf("insert snapshot: %w", err)
		}
		if err := t.insertVersionFile(ctx, tx, versionID, sd.filePath, sd.blobID, len(sd.snapshot.Body)); err != nil {
			return nil, fmt.Errorf("insert version file: %w", err)
		}
	}

	// Compute diff (best-effort)
	if parentID != "" {
		if err := t.computeAndStoreDiff(ctx, tx, parentID, versionID); err != nil {
			t.logger.Warn("Failed to compute/store combined diff", logging.Field{Key: "error", Value: err.Error()})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Working tree writes (best-effort)
	for _, sd := range list {
		_ = t.writeWorkingTreeFiles(sd.filePath, sd.snapshot.StatusCode, sd.snapshot.Body, sd.snapshot.Headers)
	}

	_ = t.writeHEAD(versionID)

	var diffID, diffJSON string
	if err := t.db.QueryRowContext(ctx, `SELECT id, diff_json FROM diffs WHERE head_version_id = ? ORDER BY created_at DESC LIMIT 1`, versionID).Scan(&diffID, &diffJSON); err != nil && err != sql.ErrNoRows {
		t.logger.Warn("failed to fetch diff row after commit batch", logging.Field{Key: "err", Value: err})
	}

	results := make([]*CommitResult, 0, len(list))
	for _, sd := range list {
		cr := &CommitResult{
			Version: Version{
				ID:         versionID,
				Parent:     parentID,
				Message:    message,
				Author:     author,
				SnapshotID: first.snapshotID,
				Timestamp:  time.Unix(ts, 0),
			},
			ParentVersionID: parentID,
			DiffID:          diffID,
			DiffJSON:        diffJSON,
			HeadBody:        sd.snapshot.Body,
			HeadBlobID:      sd.blobID,
			HeadFilePath:    sd.filePath,
			Opts:            assessor.ScoreOptions{RequestLocations: true},
		}
		results = append(results, cr)
	}

	return results, nil
}

func (t *SQLiteTracker) writeWorkingTreeFiles(filePath string, statusCode int, body []byte, headers map[string][]string) error {
	dirPath := filepath.Join(t.config.StoragePath, filePath)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	bodyPath := filepath.Join(dirPath, ".page_body")
	if err := AtomicWriteFile(bodyPath, body, 0644); err != nil {
		return fmt.Errorf("failed to write .page_body: %w", err)
	}

	headers["Status-Code"] = []string{fmt.Sprintf("%d", statusCode)}
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

func (t *SQLiteTracker) ScoreAndAttributeVersion(ctx context.Context, cr *CommitResult) error {
	if cr == nil {
		return errors.New("nil CommitResult")
	}

	if cr.DiffJSON == "" && cr.DiffID != "" {
		if err := t.db.QueryRowContext(ctx, `SELECT diff_json FROM diffs WHERE id = ?`, cr.DiffID).Scan(&cr.DiffJSON); err != nil && err != sql.ErrNoRows {
			t.logger.Warn("failed to load diff_json for commit", logging.Field{Key: "err", Value: err})
		}
	}

	if len(cr.HeadBody) == 0 && cr.HeadBlobID != "" {
		if b, err := t.store.Get(cr.HeadBlobID); err == nil {
			cr.HeadBody = b
		} else {
			t.logger.Warn("failed to load head blob for commit", logging.Field{Key: "err", Value: err})
		}
	}

	return t.scoreAndAttribute(ctx, cr.Opts, cr.Version.ID, cr.ParentVersionID, cr.DiffID, cr.DiffJSON, cr.HeadBody)
}

func (t *SQLiteTracker) scoreAndAttribute(ctx context.Context, opts assessor.ScoreOptions, versionID, parentVersionID, diffID, diffJSON string, headBody []byte) error {
	if t.assessor == nil {
		return nil
	}

	scoreCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	scoreRes, err := t.assessor.ScoreHTML(scoreCtx, headBody, fmt.Sprintf("version:%s", versionID), opts)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("scoring failed", logging.Field{Key: "version_id", Value: versionID}, logging.Field{Key: "error", Value: err})
		}
		return err
	}

	scoreJSON, err := json.Marshal(scoreRes)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to marshal score result", logging.Field{Key: "err", Value: err})
		}
		scoreJSON = []byte("{}")
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rb := tx.Rollback(); rb != nil && rb != sql.ErrTxDone {
			if t.logger != nil {
				t.logger.Warn("rollback failed", logging.Field{Key: "err", Value: rb})
			}
		}
	}()

	scoreID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO version_scores
		  (id, version_id, scoring_version, score, normalized, confidence, score_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, scoreID, versionID, scoreRes.Version, scoreRes.Score, scoreRes.Normalized, scoreRes.Confidence, string(scoreJSON), time.Now().Unix()); err != nil {
		return fmt.Errorf("insert version_scores: %w", err)
	}

	velMap, err := t.persistVersionEvidenceLocations(ctx, tx, versionID, scoreRes)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("persistVersionEvidenceLocations failed", logging.Field{Key: "err", Value: err})
		}
	}

	if parentVersionID != "" && diffID != "" && diffJSON != "" {
		if err := t.attributeUsingLocations(ctx, tx, diffID, versionID, parentVersionID, diffJSON, headBody, scoreRes, velMap); err != nil {
			if t.logger != nil {
				t.logger.Warn("attributeUsingLocations failed", logging.Field{Key: "err", Value: err})
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (t *SQLiteTracker) persistVersionEvidenceLocations(ctx context.Context, tx *sql.Tx, versionID string, scoreRes *assessor.ScoreResult) (map[string]string, error) {
	if tx == nil {
		return nil, fmt.Errorf("persistVersionEvidenceLocations requires tx")
	}
	now := time.Now().Unix()
	velMap := make(map[string]string)

	keyFor := func(evidenceID string, locIndex int) string {
		return evidenceID + ":" + strconv.Itoa(locIndex)
	}

	for ei, ev := range scoreRes.Evidence {
		eid := ev.ID
		if eid == "" {
			eid = uuid.New().String()
			scoreRes.Evidence[ei].ID = eid
		}
		evJS, _ := json.Marshal(ev)
		evJSON := string(evJS)

		if len(ev.Locations) == 0 {
			id := uuid.New().String()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO version_evidence_locations
				  (id, version_id, evidence_id, evidence_index, location_index, evidence_key, evidence_json, scoring_version, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, versionID, eid, ei, -1, ev.Key, evJSON, scoreRes.Version, now); err != nil {
				return nil, fmt.Errorf("insert version_evidence_locations global: %w", err)
			}
			velMap[keyFor(eid, -1)] = id
			continue
		}

		for li, loc := range ev.Locations {
			id := uuid.New().String()
			locJS, _ := json.Marshal(loc)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO version_evidence_locations
				  (id, version_id, evidence_id, evidence_index, location_index,
				   evidence_key, selector, xpath, node_id, file_path,
				   byte_start, byte_end, line_start, line_end,
				   loc_confidence, evidence_json, location_json, scoring_version, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, versionID, eid, ei, li,
				ev.Key,
				loc.Selector, loc.XPath, loc.NodeID, loc.FilePath,
				nullableInt(loc.ByteStart), nullableInt(loc.ByteEnd), nullableInt(loc.LineStart), nullableInt(loc.LineEnd),
				nullableFloat(loc.Confidence), evJSON, string(locJS), scoreRes.Version, now); err != nil {
				return nil, fmt.Errorf("insert version_evidence_locations: %w", err)
			}
			velMap[keyFor(eid, li)] = id
		}
	}

	return velMap, nil
}

func (t *SQLiteTracker) attributeUsingLocations(ctx context.Context, tx *sql.Tx, diffID, headVersionID, parentVersionID, diffJSON string, headBody []byte, scoreRes *assessor.ScoreResult, velMap map[string]string) error {
	if tx == nil {
		return fmt.Errorf("attributeUsingLocations requires tx")
	}

	combined := t.parseCombinedDiff(diffJSON)
	doc := t.prepareDoc(headBody)
	rows := t.collectLocRows(combined, doc, headBody, scoreRes)

	if err := t.insertAttributionRows(ctx, tx, diffID, headVersionID, parentVersionID, scoreRes, rows, velMap); err != nil {
		return err
	}

	return nil
}

type locRow struct {
	EvidenceID  string
	EvidenceKey string
	EvidenceIdx int
	LocationIdx int
	ChunkIdx    int
	Weight      float64
	EvidenceJS  string
	LocationJS  string
	Severity    string
}

func (t *SQLiteTracker) collectLocRows(combined *CombinedDiff, doc *goquery.Document, headBody []byte, scoreRes *assessor.ScoreResult) []locRow {
	var rows []locRow
	for ei, ev := range scoreRes.Evidence {
		eid := ev.ID
		if eid == "" {
			eid = uuid.New().String()
			scoreRes.Evidence[ei].ID = eid
		}
		evJS, _ := json.Marshal(ev)
		evJSON := string(evJS)

		base := severityWeight(ev.Severity) * scoreRes.Confidence
		if base <= 0 {
			base = 1.0
		}

		if len(ev.Locations) == 0 {
			rows = append(rows, locRow{
				EvidenceID:  eid,
				EvidenceKey: ev.Key,
				EvidenceIdx: ei,
				LocationIdx: -1,
				ChunkIdx:    -1,
				Weight:      base,
				EvidenceJS:  evJSON,
				LocationJS:  "",
				Severity:    ev.Severity,
			})
			continue
		}

		locConfs := make([]float64, len(ev.Locations))
		sum := 0.0
		for i, l := range ev.Locations {
			if l.Confidence != nil && *l.Confidence >= 0 {
				locConfs[i] = *l.Confidence
			} else {
				locConfs[i] = 1.0
			}
			sum += locConfs[i]
		}
		if sum == 0 {
			for i := range locConfs {
				locConfs[i] = 1.0
			}
			sum = float64(len(locConfs))
		}

		for li, loc := range ev.Locations {
			chunkIdx, _ := t.mapLocationToChunkStrict(combined, doc, headBody, loc)
			locJS, _ := json.Marshal(loc)
			rows = append(rows, locRow{
				EvidenceID:  eid,
				EvidenceKey: ev.Key,
				EvidenceIdx: ei,
				LocationIdx: li,
				ChunkIdx:    chunkIdx,
				Weight:      base * (locConfs[li] / sum),
				EvidenceJS:  evJSON,
				LocationJS:  string(locJS),
				Severity:    ev.Severity,
			})
		}
	}
	return rows
}

func (t *SQLiteTracker) insertAttributionRows(ctx context.Context, tx *sql.Tx, diffID, headVersionID, parentVersionID string, scoreRes *assessor.ScoreResult, rows []locRow, velMap map[string]string) error {
	total := 0.0
	for _, r := range rows {
		total += r.Weight
	}
	if total <= 0 {
		total = 1.0
	}

	now := time.Now().Unix()
	for _, r := range rows {
		pct := (r.Weight / total) * 100.0
		id := uuid.New().String()

		velID := ""
		if velMap != nil {
			key := r.EvidenceID + ":" + strconv.Itoa(r.LocationIdx)
			if v, ok := velMap[key]; ok {
				velID = v
			}
			if velID == "" {
				if v, ok := velMap[r.EvidenceID+":-1"]; ok {
					velID = v
				}
			}
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO diff_attributions
			  (id, diff_id, head_version_id, version_evidence_location_id, evidence_id, evidence_location_index, chunk_index, evidence_key, evidence_json, location_json, scoring_version, contribution, contribution_pct, note, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, diffID, headVersionID, nullableString(velID), r.EvidenceID, r.LocationIdx, r.ChunkIdx, r.EvidenceKey, r.EvidenceJS, r.LocationJS, scoreRes.Version, r.Weight, pct, "", now)
		if err != nil && t.logger != nil {
			t.logger.Warn("failed to insert diff_attribution", logging.Field{Key: "err", Value: err}, logging.Field{Key: "evidence_id", Value: r.EvidenceID})
		}
	}

	var parentScore sql.NullFloat64
	if err := tx.QueryRowContext(ctx, `SELECT score FROM version_scores WHERE version_id = ? ORDER BY created_at DESC LIMIT 1`, parentVersionID).Scan(&parentScore); err != nil && err != sql.ErrNoRows {
		if t.logger != nil {
			t.logger.Warn("failed to query parent score", logging.Field{Key: "err", Value: err})
		}
		parentScore = sql.NullFloat64{}
	}
	if parentScore.Valid && t.logger != nil {
		delta := scoreRes.Score - parentScore.Float64
		t.logger.Info("version scored", logging.Field{Key: "version_id", Value: headVersionID}, logging.Field{Key: "score", Value: scoreRes.Score}, logging.Field{Key: "parent_score", Value: parentScore.Float64}, logging.Field{Key: "delta", Value: delta})
	}

	return nil
}

// ---------------- mapping helpers ----------------

// mapLocationToChunkStrict maps a structured EvidenceLocation to a chunk index using only
// selector / byte range / line range matching. Returns chunk index (>=0) or -1 if none matched.
// Strength return value is unused here (kept for parity); returns 1.0 on exact match, 0.0 otherwise.
func (t *SQLiteTracker) mapLocationToChunkStrict(combined *CombinedDiff, doc *goquery.Document, headBody []byte, loc assessor.EvidenceLocation) (int, float64) {
	// prefer selector -> byte-range -> line-range
	if loc.Selector != "" && doc != nil {
		if idx, ok := t.matchSelectorToChunks(combined, doc, loc.Selector); ok {
			return idx, 1.0
		}
	}
	if loc.ByteStart != nil && loc.ByteEnd != nil && len(headBody) > 0 {
		if idx, ok := t.matchByteRangeToChunks(combined, headBody, *loc.ByteStart, *loc.ByteEnd); ok {
			return idx, 1.0
		}
	}
	if loc.LineStart != nil && loc.LineEnd != nil && len(headBody) > 0 {
		if idx, ok := t.matchLineRangeToChunks(combined, headBody, *loc.LineStart, *loc.LineEnd); ok {
			return idx, 1.0
		}
	}
	return -1, 0.0
}

// matchSelectorToChunks finds the first chunk containing the outer HTML (or text) of the first node matched by selector.
func (t *SQLiteTracker) matchSelectorToChunks(combined *CombinedDiff, doc *goquery.Document, selector string) (int, bool) {
	if doc == nil {
		return -1, false
	}
	nodes := doc.Find(selector)
	if nodes.Length() == 0 {
		return -1, false
	}
	htmlSnippet, err := nodes.First().Html()
	if err != nil || strings.TrimSpace(htmlSnippet) == "" {
		htmlSnippet = nodes.First().Text()
	}
	sn := strings.ToLower(strings.TrimSpace(htmlSnippet))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// matchByteRangeToChunks extracts the snippet from headBody and finds the first chunk containing it.
func (t *SQLiteTracker) matchByteRangeToChunks(combined *CombinedDiff, headBody []byte, start, end int) (int, bool) {
	if start < 0 {
		start = 0
	}
	if end > len(headBody) {
		end = len(headBody)
	}
	if start >= end {
		return -1, false
	}
	sn := strings.ToLower(strings.TrimSpace(string(headBody[start:end])))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// matchLineRangeToChunks extracts the lines and finds the first chunk containing that snippet.
func (t *SQLiteTracker) matchLineRangeToChunks(combined *CombinedDiff, headBody []byte, lineStart, lineEnd int) (int, bool) {
	lines := bytes.Split(headBody, []byte{'\n'})
	ls := lineStart - 1
	le := lineEnd - 1
	if ls < 0 {
		ls = 0
	}
	if le >= len(lines) {
		le = len(lines) - 1
	}
	if ls > le || ls >= len(lines) {
		return -1, false
	}
	sn := strings.ToLower(strings.TrimSpace(string(bytes.Join(lines[ls:le+1], []byte{'\n'}))))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// ---------------- small utilities ----------------

func (t *SQLiteTracker) parseCombinedDiff(diffJSON string) *CombinedDiff {
	var combined CombinedDiff
	_ = json.Unmarshal([]byte(diffJSON), &combined)
	return &combined
}

func (t *SQLiteTracker) prepareDoc(headBody []byte) *goquery.Document {
	if len(headBody) == 0 {
		return nil
	}
	if d, err := goquery.NewDocumentFromReader(bytes.NewReader(headBody)); err == nil {
		return d
	}
	return nil
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// severityWeight maps severity string to numeric weight.
func severityWeight(s string) float64 {
	switch strings.ToLower(s) {
	case "critical":
		return 5.0
	case "high":
		return 3.0
	case "medium":
		return 2.0
	case "low":
		return 1.0
	default:
		return 1.0
	}
}
