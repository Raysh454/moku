// Package demosite is the acceptance-test vocabulary for the target website
// that moku monitors. It runs the repository's demo server (cmd/demoserver)
// as a real subprocess and manipulates it only through its public /demo/*
// endpoints, the same way a person preparing a demo would.
package demosite

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/raysh454/moku/acceptance/internal/harness"
)

const startTimeout = 90 * time.Second

// Site is a running demo website process.
type Site struct {
	client *harness.Client
	proc   *harness.Process
}

// Start builds the demo server binary (once per test run), launches it on a
// free loopback port, waits for readiness, and resets its content so every
// scenario begins at page version 1.
func Start(t *testing.T) *Site {
	t.Helper()

	binary := harness.BuildBinary(t, "cmd/demoserver", "acceptance-demoserver")
	port := harness.FreePort(t)

	proc := harness.StartProcess(t, harness.ProcessSpec{
		Path: binary,
		Args: []string{fmt.Sprintf("%d", port)},
		Dir:  harness.RepoRoot(),
	})

	site := &Site{
		client: harness.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port)),
		proc:   proc,
	}
	site.waitUntilReady(t)
	site.reset(t)
	return site
}

func (s *Site) waitUntilReady(t *testing.T) {
	t.Helper()
	harness.WaitUntil(t, startTimeout, func() (bool, string, error) {
		if s.proc.Exited() {
			return false, "demo site exited before becoming ready", fmt.Errorf("process logs:\n%s", s.proc.Logs())
		}
		code, body, err := s.client.Request(http.MethodGet, "/demo/get-versions", nil, nil)
		if err != nil {
			return false, "waiting for demo site", err
		}
		if code == http.StatusOK {
			return true, "demo site ready", nil
		}
		return false, fmt.Sprintf("demo site not ready: status=%d body=%s", code, body), nil
	})
}

func (s *Site) reset(t *testing.T) {
	t.Helper()
	s.mustSucceed(t, "/demo/reset")
}

// URL returns the site's origin, the value a user enters when registering a
// website to monitor.
func (s *Site) URL() string {
	return s.client.BaseURL()
}

// WhenAllPagesBumped advances every page to its next content version — the
// "site changed" event the monitoring scenarios revolve around.
func (s *Site) WhenAllPagesBumped(t *testing.T) {
	t.Helper()
	s.mustSucceed(t, "/demo/bump-all")
}

func (s *Site) mustSucceed(t *testing.T, path string) {
	t.Helper()
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	s.client.MustStatus(t, http.StatusOK, http.MethodPost, path, nil, &resp)
	if !resp.Success {
		t.Fatalf("demo site reported failure for POST %s: %+v", path, resp)
	}
}
