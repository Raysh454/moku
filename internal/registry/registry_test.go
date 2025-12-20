package registry

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/raysh454/moku/internal/logging"
)

// helper to create temp registry (db + root dir)
func newTestRegistry(t *testing.T) (*Registry, func()) {
	t.Helper()

	rootDir := t.TempDir()
	dbPath := filepath.Join(rootDir, "moku.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	reg, err := NewRegistry(db, rootDir, logging.NewStdoutLogger("RegistryTest"))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(rootDir)
	}

	return reg, cleanup
}

func TestNormalizeOriginForDir(t *testing.T) {
	cases := []struct {
		origin string
		want   string
	}{
		{"https://example.com", "https-example.com"},
		{"http://example.com", "http-example.com"},
		{"https://example.com:8443", "https-example.com:8443"},
		{"http://example.com:8080", "http-example.com:8080"},
		{"example.com", "example.com"},
		{" example.com ", "example.com"},
	}

	for _, tc := range cases {
		got := normalizeOriginForDir(tc.origin)
		if got != tc.want {
			t.Errorf("normalizeOriginForDir(%q) = %q, want %q", tc.origin, got, tc.want)
		}
	}
}

func TestCreateProjectAndWebsite_DirectoryLayout(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Create a project
	project, err := reg.CreateProject(ctx, "", "My Project", "desc")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Ensure project dir exists
	projectDirName := normalizeNameForDir(project.Name)
	projectDirPath := filepath.Join(reg.rootDir, projectDirName)
	if _, err := os.Stat(projectDirPath); err != nil {
		t.Fatalf("expected project dir %s to exist: %v", projectDirPath, err)
	}

	// Create two websites with same host but different schemes/ports
	w1, err := reg.CreateWebsite(ctx, project.Slug, "", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite https: %v", err)
	}
	w2, err := reg.CreateWebsite(ctx, project.Slug, "", "http://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite http: %v", err)
	}
	w3, err := reg.CreateWebsite(ctx, project.Slug, "", "https://example.com:8443")
	if err != nil {
		t.Fatalf("CreateWebsite https:8443: %v", err)
	}

	// Check storage paths and directories
	expectedDirs := map[string]bool{
		"https-example.com":      false,
		"http-example.com":       false,
		"https-example.com:8443": false,
	}

	for _, w := range []*Website{w1, w2, w3} {
		if w.StoragePath == "" {
			t.Fatalf("website %s has empty StoragePath", w.Origin)
		}
		if _, err := os.Stat(w.StoragePath); err != nil {
			t.Fatalf("StoragePath %s does not exist: %v", w.StoragePath, err)
		}

		// Extract last directory component from storage path
		dirName := filepath.Base(w.StoragePath)
		if _, ok := expectedDirs[dirName]; !ok {
			t.Fatalf("unexpected website dir name %q for origin %q", dirName, w.Origin)
		}
		expectedDirs[dirName] = true
	}

	for name, seen := range expectedDirs {
		if !seen {
			t.Errorf("expected directory %q to be created", name)
		}
	}
}

func TestGetProjectBySlug_AndListProjects(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Create two projects
	p1, err := reg.CreateProject(ctx, "proj-one", "Project One", "")
	if err != nil {
		t.Fatalf("CreateProject p1: %v", err)
	}
	_, err = reg.CreateProject(ctx, "", "Project Two", "")
	if err != nil {
		t.Fatalf("CreateProject p2: %v", err)
	}

	// Get by slug
	got, err := reg.GetProjectBySlug(ctx, p1.Slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if got.ID != p1.ID || got.Name != p1.Name {
		t.Errorf("GetProjectBySlug returned %+v, want %+v", got, p1)
	}

	// List
	projects, err := reg.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("ListProjects length = %d, want 2", len(projects))
	}

	// Slugs should be normalized
	if projects[0].Slug == "" || projects[1].Slug == "" {
		t.Errorf("projects have empty slug: %+v", projects)
	}
}

func TestCreateAndGetWebsiteBySlug(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	proj, err := reg.CreateProject(ctx, "", "My Project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create website with explicit slug
	w, err := reg.CreateWebsite(ctx, proj.Slug, "my-site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	if w.Slug != "my-site" {
		t.Fatalf("expected slug 'my-site', got %q", w.Slug)
	}

	// GetWebsiteBySlug (by project slug)
	got, err := reg.GetWebsiteBySlug(ctx, proj.Slug, "my-site")
	if err != nil {
		t.Fatalf("GetWebsiteBySlug: %v", err)
	}
	if got.ID != w.ID || got.Origin != w.Origin {
		t.Errorf("GetWebsiteBySlug returned %+v, want %+v", got, w)
	}

	// GetWebsiteBySlug (by project ID)
	got2, err := reg.GetWebsiteBySlug(ctx, proj.ID, "my-site")
	if err != nil {
		t.Fatalf("GetWebsiteBySlug by project ID: %v", err)
	}
	if got2.ID != w.ID {
		t.Errorf("GetWebsiteBySlug by ID returned %+v, want %+v", got2, w)
	}
}

func TestListWebsites(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	proj, err := reg.CreateProject(ctx, "", "My Project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	_, err = reg.CreateWebsite(ctx, proj.Slug, "", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite 1: %v", err)
	}
	_, err = reg.CreateWebsite(ctx, proj.Slug, "", "http://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite 2: %v", err)
	}

	websites, err := reg.ListWebsites(ctx, proj.Slug)
	if err != nil {
		t.Fatalf("ListWebsites: %v", err)
	}
	if len(websites) != 2 {
		t.Fatalf("ListWebsites length = %d, want 2", len(websites))
	}

	// Origins should be preserved
	origins := map[string]bool{}
	for _, w := range websites {
		origins[w.Origin] = true
	}
	if !origins["https://example.com"] || !origins["http://example.com"] {
		t.Errorf("ListWebsites missing expected origins: %+v", origins)
	}
}

func TestUpdateWebsiteLastSeen(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	proj, err := reg.CreateProject(ctx, "", "My Project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	w, err := reg.CreateWebsite(ctx, proj.Slug, "", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	now := time.Unix(1700000000, 0)
	if err := reg.UpdateWebsiteLastSeen(ctx, w.ID, now); err != nil {
		t.Fatalf("UpdateWebsiteLastSeen: %v", err)
	}

	// Fetch again and check LastSeenAt
	got, err := reg.GetWebsiteBySlug(ctx, proj.Slug, w.Slug)
	if err != nil {
		t.Fatalf("GetWebsiteBySlug: %v", err)
	}
	if got.LastSeenAt != now.Unix() {
		t.Errorf("LastSeenAt = %d, want %d", got.LastSeenAt, now.Unix())
	}
}
