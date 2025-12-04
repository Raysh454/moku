package registry

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
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

// Registry manages projects and websites metadata.
type Registry struct {
	db     *sql.DB
	logger logging.Logger
}

// NewRegistry returns a Registry and runs migrations.
func NewRegistry(db *sql.DB, logger logging.Logger) (*Registry, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	// Read and execute schema
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema.sql: %w", err)
	}

	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	return &Registry{db: db, logger: logger}, nil
}

// normalizeSlug makes a slug safe (simple implementation).
func normalizeSlug(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// replace spaces with dashes, collapse multiple dashes, remove illegal chars
	s = strings.ReplaceAll(s, " ", "-")
	// keep only a-z0-9-_.
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		out = uuid.New().String()[:8]
	}
	return out
}

// CreateProject inserts a new project.
func (r *Registry) CreateProject(ctx context.Context, slug, name, description string) (*Project, error) {
	if slug == "" {
		slug = normalizeSlug(name)
	} else {
		slug = normalizeSlug(slug)
	}
	if name == "" {
		name = slug
	}
	id := uuid.New().String()
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `INSERT INTO projects (id, slug, name, description, created_at, meta) VALUES (?, ?, ?, ?, ?, ?)`,
		id, slug, name, description, now, "{}")
	if err != nil {
		return nil, err
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
	row := r.db.QueryRowContext(ctx, `SELECT id, slug, name, description, created_at, meta FROM projects WHERE slug = ? LIMIT 1`, slug)
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

// ListProjects returns all projects.
func (r *Registry) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, slug, name, description, created_at, meta FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
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
// storagePath should be absolute or relative to the daemon StorageRoot (resolve externally).
func (r *Registry) CreateWebsite(ctx context.Context, projectIdentifier string, slug string, name string, origin string, storagePath string) (*Website, error) {
	// Resolve project: try slug then id
	var projectID string
	proj, err := r.GetProjectBySlug(ctx, projectIdentifier)
	if err == ErrProjectNotFound {
		// maybe projectIdentifier is an ID
		row := r.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = ? LIMIT 1`, projectIdentifier)
		if err := row.Scan(&projectID); err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrProjectNotFound
			}
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		projectID = proj.ID
	}

	if slug == "" {
		slug = normalizeSlug(origin)
	} else {
		slug = normalizeSlug(slug)
	}
	if name == "" {
		name = slug
	}
	id := uuid.New().String()
	now := time.Now().Unix()

	// Normalize storagePath to clean absolute-ish form (caller should make absolute)
	storagePath = filepath.Clean(storagePath)

	_, err = r.db.ExecContext(ctx, `INSERT INTO websites (id, project_id, slug, name, origin, storage_path, created_at, config) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, slug, name, origin, storagePath, now, "{}")
	if err != nil {
		return nil, err
	}
	return &Website{
		ID:          id,
		ProjectID:   projectID,
		Slug:        slug,
		Name:        name,
		Origin:      origin,
		StoragePath: storagePath,
		CreatedAt:   now,
		Config:      "{}",
	}, nil
}

// GetWebsiteBySlug returns a website by project slug (or id) + website slug.
func (r *Registry) GetWebsiteBySlug(ctx context.Context, projectIdentifier, websiteSlug string) (*Website, error) {
	// resolve project id first
	var projectID string
	proj, err := r.GetProjectBySlug(ctx, projectIdentifier)
	if err == ErrProjectNotFound {
		// maybe projectIdentifier is an ID
		row := r.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = ? LIMIT 1`, projectIdentifier)
		if err := row.Scan(&projectID); err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrProjectNotFound
			}
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		projectID = proj.ID
	}

	websiteSlug = normalizeSlug(websiteSlug)
	row := r.db.QueryRowContext(ctx, `SELECT id, project_id, slug, name, origin, storage_path, created_at, last_seen_at, config FROM websites WHERE project_id = ? AND slug = ? LIMIT 1`, projectID, websiteSlug)
	var w Website
	var lastSeen sql.NullInt64
	var config sql.NullString
	if err := row.Scan(&w.ID, &w.ProjectID, &w.Slug, &w.Name, &w.Origin, &w.StoragePath, &w.CreatedAt, &lastSeen, &config); err != nil {
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
	// resolve project id first
	proj, err := r.GetProjectBySlug(ctx, projectIdentifier)
	if err == ErrProjectNotFound {
		// maybe projectIdentifier is an ID
		row := r.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = ? LIMIT 1`, projectIdentifier)
		var pid string
		if err := row.Scan(&pid); err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrProjectNotFound
			}
			return nil, err
		}
		proj = &Project{ID: pid}
	} else if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, project_id, slug, name, origin, storage_path, created_at, last_seen_at, config FROM websites WHERE project_id = ? ORDER BY created_at DESC`, proj.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Website{}
	for rows.Next() {
		var w Website
		var lastSeen sql.NullInt64
		var config sql.NullString
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Slug, &w.Name, &w.Origin, &w.StoragePath, &w.CreatedAt, &lastSeen, &config); err != nil {
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
	_, err := r.db.ExecContext(ctx, `UPDATE websites SET last_seen_at = ? WHERE id = ?`, ts.Unix(), websiteID)
	return err
}
