package registry

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/logging"
)

//go:embed schema.sql
var schemaFS embed.FS

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrWebsiteNotFound = errors.New("website not found")
)

// Registry manages projects and websites metadata in SQLite plus
// a filesystem layout under rootDir:
//
//	rootDir/
//	  <projectDirName>/                 # from project.Name
//	    <originDirName>/                # from website.Origin (scheme+host+port)
//	      tracker.db (etc.)
type Registry struct {
	db      *sql.DB
	rootDir string
	logger  logging.Logger
}

// NewRegistry returns a Registry, runs migrations from schema.sql,
// and remembers the rootDir for filesystem layout.
// db should typically be the SQLite DB at rootDir/moku.db.
func NewRegistry(db *sql.DB, rootDir string, logger logging.Logger) (*Registry, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if rootDir == "" {
		return nil, fmt.Errorf("rootDir is required")
	}
	rootDir = filepath.Clean(rootDir)
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure rootDir %s: %w", rootDir, err)
	}

	// Read and execute schema
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema.sql: %w", err)
	}
	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	return &Registry{db: db, rootDir: rootDir, logger: logger}, nil
}

// normalizeSlug makes a slug safe and simple.
func normalizeSlug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		out = uuid.New().String()[:8]
	}
	return out
}

// normalizeNameForDir normalizes a human name into a filesystem-safe directory name.
func normalizeNameForDir(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("unnamed-%d", time.Now().UnixNano())
	}
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, " ", "-")

	var b strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		out = fmt.Sprintf("unnamed-%d", time.Now().UnixNano())
	}
	return out
}

// originParts extracts scheme and host:port from an origin URL.
// e.g. "https://example.com:8443/login" -> ("https", "example.com:8443").
// If parsing fails, we treat the whole origin as "host" and use empty scheme.
func originParts(origin string) (scheme, host string) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		// Maybe it's already just a host or host:port; treat as host-only.
		return "", origin
	}
	return u.Scheme, u.Host
}

// normalizeOriginForDir converts an origin into a filesystem-safe directory name.
// We encode scheme + host:port so http/https and ports differ clearly.
//
// Examples:
//
//	"https://example.com"         -> "https-example.com"
//	"http://example.com"          -> "http-example.com"
//	"https://example.com:8443"    -> "https-example.com:8443"
//	"http://example.com:8080"     -> "http-example.com:8080"
//	"example.com"                 -> "example.com" (no scheme)
func normalizeOriginForDir(origin string) string {
	scheme, host := originParts(origin)
	if host == "" {
		// fall back to generic normalization of full string
		host = strings.TrimSpace(origin)
	}

	var base string
	if scheme != "" {
		base = scheme + "-" + host
	} else {
		base = host
	}

	base = strings.ReplaceAll(base, string(os.PathSeparator), "_")

	var b strings.Builder
	for _, r := range base {
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == ':' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		out = fmt.Sprintf("origin-%d", time.Now().UnixNano())
	}
	return out
}

// projectDir returns the absolute directory path for a project given its dir_name.
func (r *Registry) projectDir(dirName string) string {
	return filepath.Join(r.rootDir, dirName)
}

// websiteDir returns the absolute directory path for a website under a project.
func (r *Registry) websiteDir(projectDirName, websiteDirName string) string {
	return filepath.Join(r.rootDir, projectDirName, websiteDirName)
}

// CreateProject inserts a new project in the DB and creates its directory.
func (r *Registry) CreateProject(ctx context.Context, slug, name, description string) (*Project, error) {
	if name == "" && slug != "" {
		name = slug
	}
	if slug == "" && name != "" {
		slug = normalizeSlug(name)
	} else {
		slug = normalizeSlug(slug)
	}
	if name == "" {
		name = slug
	}

	dirName := normalizeNameForDir(name)
	projDir := r.projectDir(dirName)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (id, slug, name, description, created_at, meta, dir_name)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, slug, name, description, now, "{}", dirName,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	return &Project{
		ID:          id,
		Slug:        slug,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		Meta:        "{}",
	}, nil
}

// GetProjectBySlug returns a project by slug.
func (r *Registry) GetProjectBySlug(ctx context.Context, slug string) (*Project, error) {
	slug = normalizeSlug(slug)
	row := r.db.QueryRowContext(ctx,
		`SELECT id, slug, name, description, created_at, meta
         FROM projects
         WHERE slug = ?
         LIMIT 1`,
		slug,
	)
	var p Project
	var meta sql.NullString
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &meta); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	p.Meta = meta.String
	return &p, nil
}

// getProjectRowByIdentifier resolves a project either by slug or by id, and returns id + dir_name.
func (r *Registry) getProjectRowByIdentifier(ctx context.Context, identifier string) (id string, dirName string, err error) {
	// First try slug
	slug := normalizeSlug(identifier)
	row := r.db.QueryRowContext(ctx,
		`SELECT id, dir_name FROM projects WHERE slug = ? LIMIT 1`, slug)
	if err := row.Scan(&id, &dirName); err != nil {
		if err != sql.ErrNoRows {
			return "", "", err
		}
		// Try as ID
		row2 := r.db.QueryRowContext(ctx,
			`SELECT id, dir_name FROM projects WHERE id = ? LIMIT 1`, identifier)
		if err := row2.Scan(&id, &dirName); err != nil {
			if err == sql.ErrNoRows {
				return "", "", ErrProjectNotFound
			}
			return "", "", err
		}
	}
	return id, dirName, nil
}

// ListProjects returns all projects.
func (r *Registry) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, slug, name, description, created_at, meta
         FROM projects
         ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		var meta sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &meta); err != nil {
			return nil, err
		}
		p.Meta = meta.String
		out = append(out, p)
	}
	return out, nil
}

// CreateWebsite creates a website entry under a project (project slug or id accepted).
//
// Websites are stored as directories under the project directory named after
// scheme + host:port from origin:
//
//	origin: "https://example.com"       -> dir: "https-example.com"
//	origin: "http://example.com"        -> dir: "http-example.com"
//	origin: "https://example.com:8443"  -> dir: "https-example.com:8443"
//
// slug: logical identifier for the website.
// name: human-facing name; does NOT affect directory name (origin is used).
// origin: REQUIRED; used for directory name and UI label.
func (r *Registry) CreateWebsite(ctx context.Context, projectIdentifier string, slug string, origin string) (*Website, error) {
	// Resolve project id + dir_name
	projectID, projectDirName, err := r.getProjectRowByIdentifier(ctx, projectIdentifier)
	if err != nil {
		return nil, err
	}

	if origin == "" {
		return nil, fmt.Errorf("origin (URL) is required for website directory naming")
	}

	if slug == "" {
		slug = normalizeSlug(origin)
	} else {
		slug = normalizeSlug(slug)
	}

	originDirName := normalizeOriginForDir(origin)
	wdir := r.websiteDir(projectDirName, originDirName)
	if err := os.MkdirAll(wdir, 0o755); err != nil {
		return nil, fmt.Errorf("create website dir: %w", err)
	}

	absPath, err := filepath.Abs(wdir)
	if err != nil {
		absPath = wdir
	}

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO websites
             (id, project_id, slug, origin, storage_path, created_at, config, dir_name)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, slug, origin, absPath, now, "{}", originDirName,
	)
	if err != nil {
		return nil, fmt.Errorf("insert website: %w", err)
	}

	return &Website{
		ID:          id,
		ProjectID:   projectID,
		Slug:        slug,
		Origin:      origin,
		StoragePath: absPath,
		CreatedAt:   now,
		Config:      "{}",
	}, nil
}

// GetWebsiteBySlug returns a website by project slug (or id) + website slug.
func (r *Registry) GetWebsiteBySlug(ctx context.Context, projectIdentifier, websiteSlug string) (*Website, error) {
	projectID, _, err := r.getProjectRowByIdentifier(ctx, projectIdentifier)
	if err != nil {
		return nil, err
	}

	websiteSlug = normalizeSlug(websiteSlug)
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, slug, origin, storage_path, created_at, last_seen_at, config
         FROM websites
         WHERE project_id = ? AND slug = ?
         LIMIT 1`,
		projectID, websiteSlug,
	)

	var w Website
	var lastSeen sql.NullInt64
	var config sql.NullString
	if err := row.Scan(&w.ID, &w.ProjectID, &w.Slug, &w.Origin, &w.StoragePath, &w.CreatedAt, &lastSeen, &config); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrWebsiteNotFound
		}
		return nil, err
	}
	if lastSeen.Valid {
		w.LastSeenAt = lastSeen.Int64
	}
	if config.Valid {
		w.Config = config.String
	}
	return &w, nil
}

// ListWebsites lists websites for a given project (project slug or id).
func (r *Registry) ListWebsites(ctx context.Context, projectIdentifier string) ([]Website, error) {
	projectID, _, err := r.getProjectRowByIdentifier(ctx, projectIdentifier)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, slug, origin, storage_path, created_at, last_seen_at, config
         FROM websites
         WHERE project_id = ?
         ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Website
	for rows.Next() {
		var w Website
		var lastSeen sql.NullInt64
		var config sql.NullString
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Slug, &w.Origin, &w.StoragePath, &w.CreatedAt, &lastSeen, &config); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			w.LastSeenAt = lastSeen.Int64
		}
		if config.Valid {
			w.Config = config.String
		}
		out = append(out, w)
	}
	return out, nil
}

// UpdateWebsiteLastSeen updates last_seen_at for a website id.
func (r *Registry) UpdateWebsiteLastSeen(ctx context.Context, websiteID string, ts time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE websites SET last_seen_at = ? WHERE id = ?`,
		ts.Unix(), websiteID,
	)
	return err
}
