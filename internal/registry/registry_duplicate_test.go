package registry_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

func newRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	rootDir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(rootDir, "moku.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	reg, err := registry.NewRegistry(db, rootDir, logging.NewStdoutLogger("RegistryDuplicateTest"))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}

func TestCreateProject_DuplicateSlug_ReturnsSentinel(t *testing.T) {
	reg := newRegistry(t)
	ctx := context.Background()

	if _, err := reg.CreateProject(ctx, "dup", "First", ""); err != nil {
		t.Fatalf("first CreateProject: %v", err)
	}

	_, err := reg.CreateProject(ctx, "dup", "Second", "")
	if !errors.Is(err, registry.ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug on duplicate project slug, got %v", err)
	}
}

func TestCreateWebsite_DuplicateSlug_ReturnsSentinel(t *testing.T) {
	reg := newRegistry(t)
	ctx := context.Background()

	proj, err := reg.CreateProject(ctx, "proj", "Proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if _, err := reg.CreateWebsite(ctx, proj.Slug, "site", "https://example.com"); err != nil {
		t.Fatalf("first CreateWebsite: %v", err)
	}

	// Same project + same website slug, different origin so the directory and
	// origin differ but the (project_id, slug) uniqueness still collides.
	_, err = reg.CreateWebsite(ctx, proj.Slug, "site", "https://other.example.com")
	if !errors.Is(err, registry.ErrDuplicateSlug) {
		t.Fatalf("expected ErrDuplicateSlug on duplicate website slug, got %v", err)
	}
}
