package moku

import (
	"fmt"
	"net/http"
	"testing"
)

// Project is a created project, the root of moku's resource hierarchy.
type Project struct {
	server      *Server
	slug        string
	websitesIDs int
}

// GivenProject creates a project with the given slug and asserts the API
// accepted it.
func (s *Server) GivenProject(t *testing.T, slug string) *Project {
	t.Helper()

	var created struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	s.api.MustStatus(t, http.StatusCreated, http.MethodPost, "/projects", map[string]any{
		"slug":        slug,
		"name":        slug,
		"description": "acceptance scenario project",
	}, &created)

	if created.ID == "" || created.Slug != slug {
		t.Fatalf("unexpected create-project response: %+v", created)
	}
	return &Project{server: s, slug: slug}
}

// GivenWebsite registers origin as a monitored website of the project under
// an auto-assigned slug and asserts the API accepted it.
func (p *Project) GivenWebsite(t *testing.T, origin string) *Website {
	t.Helper()

	p.websitesIDs++
	slug := fmt.Sprintf("site-%d", p.websitesIDs)

	var created struct {
		ID     string `json:"id"`
		Slug   string `json:"slug"`
		Origin string `json:"origin"`
	}
	p.server.api.MustStatus(t, http.StatusCreated, http.MethodPost, p.path("/websites"), map[string]any{
		"slug":   slug,
		"origin": origin,
	}, &created)

	if created.ID == "" || created.Slug != slug || created.Origin != origin {
		t.Fatalf("unexpected create-website response: %+v", created)
	}
	return &Website{project: p, slug: slug, origin: origin}
}

// ThenListsWebsites asserts that the project's website list contains exactly
// the given website slugs, in any order.
func (p *Project) ThenListsWebsites(t *testing.T, slugs ...string) {
	t.Helper()

	var websites []struct {
		Slug string `json:"slug"`
	}
	p.server.api.MustStatus(t, http.StatusOK, http.MethodGet, p.path("/websites"), nil, &websites)

	if len(websites) != len(slugs) {
		t.Fatalf("expected %d website(s) %v, got %d: %+v", len(slugs), slugs, len(websites), websites)
	}
	listed := make(map[string]bool, len(websites))
	for _, website := range websites {
		listed[website.Slug] = true
	}
	for _, slug := range slugs {
		if !listed[slug] {
			t.Fatalf("expected website %q in list, got %+v", slug, websites)
		}
	}
}

func (p *Project) path(suffix string) string {
	return "/projects/" + p.slug + suffix
}
