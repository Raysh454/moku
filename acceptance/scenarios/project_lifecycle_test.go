package scenarios_test

import (
	"testing"

	"github.com/raysh454/moku/acceptance/demosite"
	"github.com/raysh454/moku/acceptance/moku"
)

// Given a running server and a reachable target site, creating a project
// with one monitored website must make both visible through the list APIs.
func TestProjectLifecycle_CreatedProjectAndWebsiteAreListed(t *testing.T) {
	server := moku.Start(t)
	target := demosite.Start(t)

	project := server.GivenProject(t, "acme")
	site := project.GivenWebsite(t, target.URL())

	server.ThenListsProjects(t, "acme")
	project.ThenListsWebsites(t, site.Slug())
}
