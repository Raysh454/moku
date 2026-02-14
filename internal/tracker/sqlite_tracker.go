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
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/blobstore"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/tracker/score"
	"github.com/raysh454/moku/internal/utils"
	_ "modernc.org/sqlite"
)

var (
	ErrProjectIDEmpty    = errors.New("project id is empty")
	ErrProjectIDMismatch = errors.New("project id mismatch")
)

type SQLiteTracker struct {
	db       *sql.DB
	store    *blobstore.Blobstore
	logger   logging.Logger
	config   *Config
	assessor assessor.Assessor
	score    *score.SQLiteScoreTracker
}

func NewSQLiteTracker(config *Config, logger logging.Logger, assessor assessor.Assessor) (*SQLiteTracker, error) {
	if logger == nil {
		return nil, errors.New("tracker: nil logger provided")
	}

	if config == nil {
		config = &Config{}
	}

	mokuDir := filepath.Join(config.StoragePath, ".moku")
	if _, err := os.Stat(mokuDir); err != nil && config.ProjectID == "" {
		return nil, fmt.Errorf("storage path .moku does not exist; must provide project_id to initialize new tracker at %s", mokuDir)
	}

	if err := os.MkdirAll(mokuDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .moku directory: %w", err)
	}

	dbPath := filepath.Join(mokuDir, "moku.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	blobsDir := filepath.Join(mokuDir, "blobs")
	store, err := blobstore.New(blobsDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create FSStore: %w", err)
	}

	logger.Info("SQLiteTracker initialized", logging.Field{Key: "config.StoragePath", Value: config.StoragePath})

	t := &SQLiteTracker{
		db:       db,
		store:    store,
		logger:   logger,
		config:   config,
		assessor: assessor,
		score:    score.New(assessor, db, logger),
	}

	if config.ProjectID != "" {
		if err := t.SetProjectID(context.Background(), config.ProjectID, config.ForceProjectID); err != nil {
			db.Close()
			return nil, fmt.Errorf("project id mismatch or set failed: %w", err)
		}
	}

	return t, nil
}

func (t *SQLiteTracker) SetAssessor(a assessor.Assessor) {
	t.assessor = a
}

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
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?)`, "project_id", projectID); err != nil {
			return fmt.Errorf("insert meta: %w", err)
		}
	} else {
		if existing.String == projectID {
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit: %w", err)
			}
			return nil
		}
		if !force {
			return ErrProjectIDMismatch
		}
		if _, err := tx.ExecContext(ctx, `UPDATE meta SET value = ? WHERE key = ?`, projectID, "project_id"); err != nil {
			return fmt.Errorf("update meta: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

var _ Tracker = (*SQLiteTracker)(nil)

func (t *SQLiteTracker) Commit(ctx context.Context, snapshot *models.Snapshot, message string, author string) (*models.CommitResult, error) {
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

	parentID, err := t.ReadHEAD()
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

	redactSensitive := t.config != nil && t.config.RedactSensitiveHeaders
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

	if err := t.insertVersion(ctx, tx, versionID, parentID, message, author, timestamp); err != nil {
		return nil, fmt.Errorf("failed to insert version: %w", err)
	}

	if err := t.insertSnapshot(ctx, tx, snapshotData{
		snapshot:    &models.Snapshot{ID: snapshotID, StatusCode: snapshot.StatusCode, URL: snapshot.URL, Headers: normalizedHeaders, CreatedAt: time.Unix(timestamp, 0)},
		snapshotID:  snapshotID,
		versionID:   versionID,
		blobID:      blobID,
		filePath:    filePath,
		headersJSON: string(headersJSON),
	}); err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
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

	return &models.CommitResult{
		Version:         models.Version{ID: versionID, Parent: parentID, Message: message, Author: author, Timestamp: time.Unix(timestamp, 0)},
		ParentVersionID: parentID,
		DiffID:          diffID,
		DiffJSON:        diffJSON,
		Snapshots:       []*models.Snapshot{{ID: snapshotID, VersionID: versionID, StatusCode: snapshot.StatusCode, URL: snapshot.URL, Body: snapshot.Body, Headers: normalizedHeaders, CreatedAt: time.Unix(timestamp, 0)}},
	}, nil
}

func (t *SQLiteTracker) diffFromCache(diffJSON string) (*models.CombinedMultiDiff, error) {
	var combinedDiff models.CombinedMultiDiff
	if err := json.Unmarshal([]byte(diffJSON), &combinedDiff); err != nil {
		return nil, fmt.Errorf("failed to unmarshal combined multi diff: %w", err)
	}
	return &combinedDiff, nil
}

func (t *SQLiteTracker) parseHeadersJSON(headersJSON string, logFields ...logging.Field) map[string][]string {
	var headers map[string][]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		if t.logger != nil {
			fields := append([]logging.Field{{Key: "error", Value: err.Error()}}, logFields...)
			t.logger.Warn("Failed to parse headers", fields...)
		}
		return nil
	}
	return headers
}

func buildSnapshot(
	snapshotID, versionID string,
	statusCode int,
	url string,
	body []byte,
	headers map[string][]string,
	createdAtUnix int64,
) *models.Snapshot {
	return &models.Snapshot{
		ID:         snapshotID,
		VersionID:  versionID,
		StatusCode: statusCode,
		URL:        url,
		Body:       body,
		Headers:    headers,
		CreatedAt:  time.Unix(createdAtUnix, 0),
	}
}

func (t *SQLiteTracker) computeDiff(ctx context.Context, tx *sql.Tx, baseID, headID string) (*models.CombinedMultiDiff, error) {
	baseSnaps, err := t.getVersionSnapshots(ctx, tx, baseID)
	if err != nil && baseID != "" {
		return nil, fmt.Errorf("failed to get base version snapshots: %w", err)
	}
	headSnaps, err := t.getVersionSnapshots(ctx, tx, headID)
	if err != nil {
		return nil, fmt.Errorf("failed to get head version snapshots: %w", err)
	}

	redactSensitive := t.config != nil && t.config.RedactSensitiveHeaders

	files := make([]models.CombinedFileDiff, 0)
	for filePath, headSnapshot := range headSnaps {
		var baseBody []byte
		var baseHeaders map[string][]string
		if baseSnapshot, ok := baseSnaps[filePath]; ok {
			baseBody = baseSnapshot.body
			baseHeaders = baseSnapshot.headers
		}
		bodyDiffJSON, err := computeTextDiffJSON(baseID, headID, baseBody, headSnapshot.body)
		if err != nil {
			return nil, fmt.Errorf("failed to compute body diff for %s: %w", filePath, err)
		}
		var bodyDiff models.BodyDiff
		if err := json.Unmarshal([]byte(bodyDiffJSON), &bodyDiff); err != nil {
			return nil, fmt.Errorf("failed to unmarshal body diff for %s: %w", filePath, err)
		}
		headersDiff := diffHeaders(baseHeaders, headSnapshot.headers, redactSensitive)
		files = append(files, models.CombinedFileDiff{FilePath: filePath, BodyDiff: bodyDiff, HeadersDiff: headersDiff})
	}

	combinedDiff := models.CombinedMultiDiff{BaseVersionID: baseID, HeadVersionID: headID, Files: files}
	return &combinedDiff, nil
}

func (t *SQLiteTracker) DiffVersions(ctx context.Context, baseID, headID string) (*models.CombinedMultiDiff, error) {
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

	tx, err := t.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	combinedDiff, err := t.computeDiff(ctx, tx, baseID, headID)
	if err != nil {
		return nil, err
	}

	diffJSONBytes, err := json.Marshal(combinedDiff)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal combined diff: %w", err)
	}
	diffJSON = string(diffJSONBytes)

	diffID := uuid.New().String()
	_, err = t.db.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, diffJSON, time.Now().Unix())
	if err != nil {
		t.logger.Warn("Failed to cache diff", logging.Field{Key: "error", Value: err.Error()})
	}

	return combinedDiff, nil
}

func (t *SQLiteTracker) DiffSnapshots(ctx context.Context, baseSnapshotID, headSnapshotID string) (*models.CombinedFileDiff, error) {
	t.logger.Debug("Computing snapshot diff", logging.Field{Key: "baseSnapshotID", Value: baseSnapshotID}, logging.Field{Key: "headSnapshotID", Value: headSnapshotID})

	baseSnap, err := t.GetSnapshot(ctx, baseSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base snapshot: %w", err)
	}
	headSnap, err := t.GetSnapshot(ctx, headSnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get head snapshot: %w", err)
	}
	if baseSnap.URL != headSnap.URL {
		return nil, fmt.Errorf("snapshot URL mismatch: base(%s) != head(%s)", baseSnap.URL, headSnap.URL)
	}
	vDiff, err := t.DiffVersions(ctx, baseSnap.VersionID, headSnap.VersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to diff versions: %w", err)
	}
	for _, fileDiff := range vDiff.Files {
		urlTools, err := utils.NewURLTools(headSnap.URL)
		if err != nil {
			t.logger.Warn("Failed to parse snapshot URL during diff", logging.Field{Key: "error", Value: err.Error()})
			continue
		}
		if fileDiff.FilePath == urlTools.GetPath() {
			return &fileDiff, nil
		}
	}

	return nil, fmt.Errorf("no diff found for snapshot URL: %s", headSnap.URL)
}

func (t *SQLiteTracker) GetSnapshot(ctx context.Context, snapshotID string) (*models.Snapshot, error) {
	t.logger.Debug("Getting snapshot", logging.Field{Key: "snapshotID", Value: snapshotID})
	var (
		snapshotVersionID, url, filePath, blobID string
		createdAt                                int64
		statusCode                               int
		headersJSONSQL                           sql.NullString
	)

	err := t.db.QueryRowContext(ctx, `
		SELECT s.version_id, s.status_code, s.url, s.file_path, s.blob_id, s.created_at, s.headers
		FROM snapshots s
		WHERE s.id = ?
	`, snapshotID).Scan(&snapshotVersionID, &statusCode, &url, &filePath, &blobID, &createdAt, &headersJSONSQL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
		}
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}
	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob %s: %w", blobID, err)
	}
	var headers map[string][]string
	if headersJSONSQL.Valid {
		headers = t.parseHeadersJSON(headersJSONSQL.String, logging.Field{Key: "snapshot_id", Value: snapshotID})
	} else {
		headers = make(map[string][]string)
	}
	return buildSnapshot(snapshotID, snapshotVersionID, statusCode, url, body, headers, createdAt), nil
}

func (t *SQLiteTracker) GetSnapshots(ctx context.Context, versionID string) ([]*models.Snapshot, error) {
	t.logger.Debug("Getting snapshots", logging.Field{Key: "versionID", Value: versionID})

	rows, err := t.db.QueryContext(ctx, `
		SELECT s.id, s.version_id, s.status_code, s.url, s.file_path, s.blob_id, s.created_at, s.headers
		FROM snapshots s
		WHERE s.version_id = ?
		ORDER BY s.file_path
	`, versionID)

	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*models.Snapshot
	for rows.Next() {
		var snapshotID, snapshotVersionID, url, filePath, blobID string
		var createdAt int64
		var statusCode int
		var headersJSONSQL sql.NullString

		if err := rows.Scan(&snapshotID, &snapshotVersionID, &statusCode, &url, &filePath, &blobID, &createdAt, &headersJSONSQL); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}

		body, err := t.store.Get(blobID)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob %s: %w", blobID, err)
		}

		headersJSON := headersJSONSQL.String
		headers := t.parseHeadersJSON(headersJSON, logging.Field{Key: "snapshot_id", Value: snapshotID})

		snapshots = append(snapshots, buildSnapshot(snapshotID, snapshotVersionID, statusCode, url, body, headers, createdAt))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("version not found or has no snapshots: %s", versionID)
	}

	return snapshots, nil
}

func (t *SQLiteTracker) GetSnapshotByURL(ctx context.Context, url string) (*models.Snapshot, error) {
	t.logger.Debug("Getting snapshot by URL", logging.Field{Key: "url", Value: url})

	var (
		snapshotID     string
		versionID      string
		blobID         string
		statusCode     int
		createdAtUnix  int64
		headersJSONSQL sql.NullString
	)

	err := t.db.QueryRowContext(ctx, `
		SELECT s.id, s.version_id, s.status_code, s.blob_id, s.created_at, s.headers
		FROM snapshots s
		WHERE s.url = ?
		ORDER BY s.created_at DESC
		LIMIT 1
	`, url).Scan(&snapshotID, &versionID, &statusCode, &blobID, &createdAtUnix, &headersJSONSQL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found for URL: %s", url)
		}
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob %s: %w", blobID, err)
	}

	var headers map[string][]string
	if headersJSONSQL.Valid {
		headers = t.parseHeadersJSON(headersJSONSQL.String, logging.Field{Key: "snapshot_id", Value: snapshotID})
	} else {
		headers = make(map[string][]string)
	}

	return buildSnapshot(snapshotID, versionID, statusCode, url, body, headers, createdAtUnix), nil
}

func (t *SQLiteTracker) GetSnapshotByURLAndVersionID(ctx context.Context, url, versionID string) (*models.Snapshot, error) {
	t.logger.Debug("Getting snapshot by URL and VersionID", logging.Field{Key: "url", Value: url}, logging.Field{Key: "versionID", Value: versionID})

	var (
		snapshotID     string
		blobID         string
		statusCode     int
		createdAtUnix  int64
		headersJSONSQL sql.NullString
	)

	err := t.db.QueryRowContext(ctx, `
		SELECT s.id, s.status_code, s.blob_id, s.created_at, s.headers
		FROM snapshots s
		WHERE s.url = ? AND s.version_id = ?
		LIMIT 1
	`, url, versionID).Scan(&snapshotID, &statusCode, &blobID, &createdAtUnix, &headersJSONSQL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found for URL %s and version %s", url, versionID)
		}
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	body, err := t.store.Get(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob %s: %w", blobID, err)
	}

	var headers map[string][]string
	if headersJSONSQL.Valid {
		headers = t.parseHeadersJSON(headersJSONSQL.String, logging.Field{Key: "snapshot_id", Value: snapshotID})
	} else {
		headers = make(map[string][]string)
	}

	return buildSnapshot(snapshotID, versionID, statusCode, url, body, headers, createdAtUnix), nil
}

func (t *SQLiteTracker) ListVersions(ctx context.Context, limit int) ([]*models.Version, error) {
	t.logger.Debug("Listing versions", logging.Field{Key: "limit", Value: limit})

	if limit <= 0 {
		limit = 10
	}

	rows, err := t.db.QueryContext(ctx, `
		SELECT id, parent_id, message, author, timestamp
		FROM versions
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	var versions []*models.Version
	for rows.Next() {
		var v models.Version
		var parentID, author sql.NullString
		var timestamp int64

		if err := rows.Scan(&v.ID, &parentID, &v.Message, &author, &timestamp); err != nil {
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

func (t *SQLiteTracker) GetParentVersionID(ctx context.Context, versionID string) (string, error) {
	t.logger.Debug("Getting parent version ID", logging.Field{Key: "versionID", Value: versionID})

	var parentID sql.NullString
	err := t.db.QueryRowContext(ctx, `
		SELECT parent_id FROM versions WHERE id = ?
	`, versionID).Scan(&parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("version not found: %s", versionID)
		}
		return "", fmt.Errorf("failed to query parent version ID: %w", err)
	}
	if !parentID.Valid {
		return "", nil
	}
	return parentID.String, nil
}

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
		SELECT s.file_path, s.blob_id
		FROM snapshots s
		WHERE s.version_id = ?
	`, versionID)
	if err != nil {
		return fmt.Errorf("failed to query version snapshots: %w", err)
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
		return fmt.Errorf("error iterating version snapshots: %w", err)
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
			WHERE s.version_id = ? AND s.file_path = ?
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

func (t *SQLiteTracker) DB() *sql.DB {
	return t.db
}

func (t *SQLiteTracker) Close() error {
	t.logger.Info("Closing SQLiteTracker")
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *SQLiteTracker) computeAndStoreDiff(ctx context.Context, tx *sql.Tx, baseID, headID string) error {
	multi, err := t.computeDiff(ctx, tx, baseID, headID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(multi)
	if err != nil {
		return fmt.Errorf("failed to marshal multi-file combined diff: %w", err)
	}

	diffID := uuid.New().String()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO diffs (id, base_version_id, head_version_id, diff_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, diffID, nullableString(baseID), headID, string(data), time.Now().Unix())
	return err
}

type snapshotRec struct {
	body    []byte
	headers map[string][]string
}

func (t *SQLiteTracker) getVersionSnapshots(ctx context.Context, tx *sql.Tx, versionID string) (map[string]snapshotRec, error) {
	res := make(map[string]snapshotRec)
	if versionID == "" {
		return res, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT s.file_path, s.blob_id, s.headers
		FROM snapshots s
		WHERE s.version_id = ?
	`, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query version snapshots: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var path, blobID string
		var headersJSON sql.NullString
		if err := rows.Scan(&path, &blobID, &headersJSON); err != nil {
			return nil, fmt.Errorf("failed to scan version snapshot: %w", err)
		}
		body, err := t.store.Get(blobID)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob: %w", err)
		}
		res[path] = snapshotRec{body: body, headers: t.parseHeaders(headersJSON)}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating version snapshots: %w", err)
	}
	return res, nil
}

func (t *SQLiteTracker) HEADExists() (bool, error) {
	headPath := filepath.Join(t.config.StoragePath, ".moku", "HEAD")
	_, err := os.Stat(headPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (t *SQLiteTracker) ReadHEAD() (string, error) {
	headPath := filepath.Join(t.config.StoragePath, ".moku", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (t *SQLiteTracker) writeHEAD(versionID string) error {
	headPath := filepath.Join(t.config.StoragePath, ".moku", "HEAD")
	return blobstore.AtomicWriteFile(headPath, []byte(versionID), 0644)
}

func nullableString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

type snapshotData struct {
	snapshot    *models.Snapshot
	snapshotID  string
	versionID   string
	blobID      string
	filePath    string
	headersJSON string
}

func (t *SQLiteTracker) insertSnapshot(ctx context.Context, tx *sql.Tx, sd snapshotData) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO snapshots (id, version_id, status_code, url, file_path, blob_id, created_at, headers)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `,
		sd.snapshotID,
		sd.versionID,
		sd.snapshot.StatusCode,
		sd.snapshot.URL,
		sd.filePath,
		sd.blobID,
		sd.snapshot.CreatedAt.Unix(),
		sd.headersJSON,
	)
	return err
}

func (t *SQLiteTracker) insertVersion(ctx context.Context, tx *sql.Tx,
	versionID, parentID, message, author string, ts int64) error {

	_, err := tx.ExecContext(ctx, `
        INSERT INTO versions (id, parent_id, message, author, timestamp)
        VALUES (?, ?, ?, ?, ?)
    `,
		versionID,
		nullableString(parentID),
		message,
		nullableString(author),
		ts,
	)
	return err
}

func (t *SQLiteTracker) CommitBatch(ctx context.Context, snapshots []*models.Snapshot, message, author string) (*models.CommitResult, error) {
	if len(snapshots) == 0 {
		return nil, errors.New("no snapshots to commit")
	}
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	t.logger.Info("Starting batch commit", logging.Field{Key: "count", Value: len(snapshots)})

	var batchSnapshots []snapshotData
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

		batchSnapshots = append(batchSnapshots, snapshotData{
			snapshot:    snap,
			snapshotID:  uuid.New().String(),
			blobID:      blobID,
			filePath:    filePath,
			headersJSON: string(headersJSONBytes),
		})
	}

	if len(batchSnapshots) == 0 {
		return nil, errors.New("no valid snapshots to commit")
	}

	versionID := uuid.New().String()
	commitTimestamp := time.Now().Unix()

	parentID, _ := t.ReadHEAD()

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			t.logger.Warn("Failed to rollback transaction", logging.Field{Key: "error", Value: rbErr.Error()})
		}
	}()

	if err := t.insertVersion(ctx, tx, versionID, parentID, message, author, commitTimestamp); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	for _, snapshotRecord := range batchSnapshots {
		snapshotRecord.versionID = versionID
		if err := t.insertSnapshot(ctx, tx, snapshotRecord); err != nil {
			return nil, fmt.Errorf("insert snapshot: %w", err)
		}
	}

	if parentID != "" {
		if err := t.computeAndStoreDiff(ctx, tx, parentID, versionID); err != nil {
			t.logger.Warn("Failed to compute/store combined diff", logging.Field{Key: "error", Value: err.Error()})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	for _, snapshotRecord := range batchSnapshots {
		_ = t.writeWorkingTreeFiles(snapshotRecord.filePath, snapshotRecord.snapshot.StatusCode, snapshotRecord.snapshot.Body, snapshotRecord.snapshot.Headers)
	}

	_ = t.writeHEAD(versionID)

	var diffID, diffJSON string
	if err := t.db.QueryRowContext(ctx, `SELECT id, diff_json FROM diffs WHERE head_version_id = ? ORDER BY created_at DESC LIMIT 1`, versionID).Scan(&diffID, &diffJSON); err != nil && err != sql.ErrNoRows {
		t.logger.Warn("failed to fetch diff row after commit batch", logging.Field{Key: "err", Value: err})
	}

	cr := t.createCommitResult(batchSnapshots, versionID, parentID, message, author, commitTimestamp, diffID, diffJSON)
	return cr, nil
}

func (*SQLiteTracker) createCommitResult(batchSnapshots []snapshotData, versionID string, parentID string, message string, author string, commitTimestamp int64, diffID string, diffJSON string) *models.CommitResult {
	snaps := make([]*models.Snapshot, 0, len(batchSnapshots))
	for _, snapshotRecord := range batchSnapshots {
		snaps = append(snaps, &models.Snapshot{
			ID:         snapshotRecord.snapshotID,
			VersionID:  versionID,
			StatusCode: snapshotRecord.snapshot.StatusCode,
			URL:        snapshotRecord.snapshot.URL,
			Body:       snapshotRecord.snapshot.Body,
			Headers:    snapshotRecord.snapshot.Headers,
			CreatedAt:  snapshotRecord.snapshot.CreatedAt,
		})
	}
	cr := &models.CommitResult{
		Version: models.Version{
			ID:        versionID,
			Parent:    parentID,
			Message:   message,
			Author:    author,
			Timestamp: time.Unix(commitTimestamp, 0),
		},
		ParentVersionID: parentID,
		DiffID:          diffID,
		DiffJSON:        diffJSON,
		Snapshots:       snaps,
	}
	return cr
}

func (t *SQLiteTracker) writeWorkingTreeFiles(filePath string, statusCode int, body []byte, headers map[string][]string) error {
	dirPath := filepath.Join(t.config.StoragePath, filePath)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	bodyPath := filepath.Join(dirPath, ".page_body")
	if err := blobstore.AtomicWriteFile(bodyPath, body, 0644); err != nil {
		return fmt.Errorf("failed to write .page_body: %w", err)
	}

	headers["Status-Code"] = []string{fmt.Sprintf("%d", statusCode)}
	headersJSON, err := json.MarshalIndent(headers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}
	headersPath := filepath.Join(dirPath, ".page_headers.json")
	if err := blobstore.AtomicWriteFile(headersPath, headersJSON, 0644); err != nil {
		return fmt.Errorf("failed to write .page_headers.json: %w", err)
	}

	return nil
}

func (t *SQLiteTracker) ScoreAndAttributeVersion(ctx context.Context, cr *models.CommitResult, scoreTimeout time.Duration) error {
	if cr == nil {
		return errors.New("nil CommitResult")
	}

	if t.assessor == nil {
		return errors.New("no assessor set on tracker")
	}

	t.logger.Info("Starting scoring and attribution")

	return t.score.ScoreAndAttribute(ctx, cr, scoreTimeout)
}

func (t *SQLiteTracker) GetScoreResultFromSnapshotID(ctx context.Context, snapshotID string) (*assessor.ScoreResult, error) {
	return t.score.GetScoreResultFromSnapshotID(ctx, snapshotID)
}

func (t *SQLiteTracker) GetScoreResultsFromVersionID(ctx context.Context, versionID string) ([]*assessor.ScoreResult, error) {
	return t.score.GetScoreResultsFromVersionID(ctx, versionID)
}

func (t *SQLiteTracker) GetSecurityDiffOverview(ctx context.Context, baseID, headID string) (*assessor.SecurityDiffOverview, error) {
	return t.score.GetSecurityDiffOverview(ctx, baseID, headID)
}

func (t *SQLiteTracker) GetSecurityDiff(ctx context.Context, baseSnapshotID, headSnapshotID string) (*assessor.SecurityDiff, error) {
	return t.score.GetSecurityDiff(ctx, baseSnapshotID, headSnapshotID)
}
