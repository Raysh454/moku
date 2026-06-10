// Package scenarios contains Moku's executable acceptance specifications.
//
// Every spec in this package observes the system strictly from the outside:
// real binaries spawned as subprocesses, configured only through their public
// surface (CLI args and MOKU_* environment variables), and exercised only
// over HTTP. The specs may import the moku and demosite vocabulary packages —
// nothing else from this repository.
package scenarios_test

import (
	"testing"

	"github.com/raysh454/moku/acceptance/moku"
)

// Given a freshly built moku binary started against an empty storage root,
// the API must come up and serve an empty project list.
func TestServerBoot_ServesEmptyProjectList(t *testing.T) {
	server := moku.Start(t)

	server.ThenListsProjects(t)
}
