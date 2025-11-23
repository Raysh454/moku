package registry_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		t.Logf("pragmas: %v", err)
	}
	return db
}

func TestRegistry_CreateListProjectAndWebsite(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	logger := logging.NewStdoutLogger("registry_test")
	reg, err := registry.NewRegistry(db, logger)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	ctx := context.Background()

	// create project
	proj, err := reg.CreateProject(ctx, "myproj", "My Project", "desc")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.Slug != "myproj" {
		t.Fatalf("unexpected project slug: %s", proj.Slug)
	}

	// list projects
	projects, err := reg.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project got %d", len(projects))
	}

	// create website
	site, err := reg.CreateWebsite(ctx, proj.Slug, "site1", "Site 1", "https://example.com", "/tmp/example-site")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if site.Slug != "site1" {
		t.Fatalf("unexpected site slug: %s", site.Slug)
	}

	// get website
	got, err := reg.GetWebsiteBySlug(ctx, proj.Slug, site.Slug)
	if err != nil {
		t.Fatalf("GetWebsiteBySlug: %v", err)
	}
	if got.ID != site.ID {
		t.Fatalf("GetWebsiteBySlug returned wrong site id, want %s got %s", site.ID, got.ID)
	}
}

func TestRegistry_UpdateLastSeen(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	logger := logging.NewStdoutLogger("registry_test")
	reg, err := registry.NewRegistry(db, logger)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	ctx := context.Background()
	proj, err := reg.CreateProject(ctx, "", "p", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	site, err := reg.CreateWebsite(ctx, proj.Slug, "s", "", "https://example.org", "/tmp/x")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if err := reg.UpdateWebsiteLastSeen(ctx, site.ID, time.Now()); err != nil {
		t.Fatalf("UpdateWebsiteLastSeen: %v", err)
	}
	// fetch back and ensure last_seen_at > 0
	got, err := reg.GetWebsiteBySlug(ctx, proj.Slug, site.Slug)
	if err != nil {
		t.Fatalf("GetWebsiteBySlug: %v", err)
	}
	if got.LastSeenAt == 0 {
		t.Fatalf("expected last_seen_at set, got 0")
	}
}
