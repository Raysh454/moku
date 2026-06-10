// Package moku is the acceptance-test vocabulary for the system under test.
// It starts the real moku binary as a subprocess, configures it exclusively
// through its public surface (CLI args and MOKU_* environment variables), and
// exposes fluent Given/When/Then verbs that speak plain HTTP — exactly what a
// user or the React frontend can do, and nothing more.
package moku

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/raysh454/moku/acceptance/internal/harness"
)

const startTimeout = 90 * time.Second

// Server is a running moku API process.
type Server struct {
	api  *harness.Client
	proc *harness.Process
}

// Start builds the moku binary (once per test run), launches it on a free
// loopback port against a throwaway storage root, and blocks until the API
// answers. The private-hosts escape hatch is enabled the same way a user
// would enable it, because scenarios fetch a loopback demo site.
func Start(t *testing.T) *Server {
	t.Helper()

	binary := harness.BuildBinary(t, ".", "acceptance-moku")
	port := harness.FreePort(t)

	proc := harness.StartProcess(t, harness.ProcessSpec{
		Path: binary,
		Args: []string{"127.0.0.1", fmt.Sprintf("%d", port)},
		Env: []string{
			"MOKU_STORAGE_ROOT=" + t.TempDir(),
			"MOKU_ALLOW_PRIVATE_HOSTS=1",
		},
		Dir: harness.RepoRoot(),
	})

	server := &Server{
		api:  harness.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port)),
		proc: proc,
	}
	server.waitUntilReady(t)
	return server
}

func (s *Server) waitUntilReady(t *testing.T) {
	t.Helper()
	harness.WaitUntil(t, startTimeout, func() (bool, string, error) {
		if s.proc.Exited() {
			return false, "moku exited before becoming ready", fmt.Errorf("process logs:\n%s", s.proc.Logs())
		}
		code, body, err := s.api.Request(http.MethodGet, "/projects", nil, nil)
		if err != nil {
			return false, "waiting for moku API", err
		}
		if code == http.StatusOK {
			return true, "moku API ready", nil
		}
		return false, fmt.Sprintf("moku API not ready: status=%d body=%s", code, body), nil
	})
}

// BaseURL returns the API's base URL.
func (s *Server) BaseURL() string {
	return s.api.BaseURL()
}

// ThenListsProjects asserts that GET /projects returns exactly the given
// project slugs, in any order. Call it with no arguments to assert the list
// is empty.
func (s *Server) ThenListsProjects(t *testing.T, slugs ...string) {
	t.Helper()

	var projects []struct {
		Slug string `json:"slug"`
	}
	s.api.MustStatus(t, http.StatusOK, http.MethodGet, "/projects", nil, &projects)

	if len(projects) != len(slugs) {
		t.Fatalf("expected %d project(s) %v, got %d: %+v", len(slugs), slugs, len(projects), projects)
	}
	listed := make(map[string]bool, len(projects))
	for _, project := range projects {
		listed[project.Slug] = true
	}
	for _, slug := range slugs {
		if !listed[slug] {
			t.Fatalf("expected project %q in list, got %+v", slug, projects)
		}
	}
}
