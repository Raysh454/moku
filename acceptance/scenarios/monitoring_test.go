package scenarios_test

import (
	"testing"

	"github.com/raysh454/moku/acceptance/demosite"
	"github.com/raysh454/moku/acceptance/moku"
)

// The product's core promise: when a monitored site changes, moku surfaces
// the new content version and flags the changed attack surface.
func TestMonitoring_VersionBumpChangesAttackSurface(t *testing.T) {
	server := moku.Start(t)
	target := demosite.Start(t)

	project := server.GivenProject(t, "acme")
	site := project.GivenWebsite(t, target.URL())

	site.WhenEnumerated(t)
	site.WhenFetched(t)

	site.ThenHasEndpoint(t, "/")

	home := site.Endpoint("/")
	v1 := home.Snapshot(t)
	v1.ThenServedOK(t)
	v1.ThenBodyContains(t, "Version 1 - Basic home page")

	target.WhenAllPagesBumped(t)
	site.WhenFetched(t)

	v2 := home.WaitForNewVersion(t, v1)
	v2.ThenBodyContains(t, "Version 2 - Added admin and upload links")
	v2.ThenBodyDiffRecorded(t)
	v2.ThenAttackSurfaceChangedSince(t, v1)
	v2.ThenHasSecurityHeader(t, "x-content-type-options", "nosniff")
}
