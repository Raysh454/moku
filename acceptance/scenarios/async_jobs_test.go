package scenarios_test

import (
	"testing"

	"github.com/raysh454/moku/acceptance/demosite"
	"github.com/raysh454/moku/acceptance/moku"
)

// Jobs are asynchronous: starting a fetch returns a handle immediately, and
// completion is reported over the same SSE stream the frontend consumes.
func TestAsyncJobs_StartedFetchReportsCompletionOverSSE(t *testing.T) {
	server := moku.Start(t)
	target := demosite.Start(t)

	project := server.GivenProject(t, "acme")
	site := project.GivenWebsite(t, target.URL())
	site.WhenEnumerated(t)

	job := site.WhenFetchStarted(t)

	job.ThenSucceeds(t)
	site.ThenHasEndpoint(t, "/")
}

// A target that cannot be reached must surface as a failed job — never a
// silent success and never a hang.
func TestAsyncJobs_UnreachableTargetFailsTheEnumerationJob(t *testing.T) {
	server := moku.Start(t)

	project := server.GivenProject(t, "ghost")
	site := project.GivenWebsite(t, "http://127.0.0.1:9") // discard port: nothing listens

	job := site.WhenEnumerateStarted(t)

	job.ThenFails(t)
}
