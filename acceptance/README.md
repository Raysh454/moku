# Acceptance suite

Black-box acceptance tests for moku. Every scenario here observes the system
the way a user does: the **real binaries** are spawned as subprocesses,
configured only through their public surface (CLI args and `MOKU_*`
environment variables), and exercised only over **HTTP + SSE** — the same wire
protocol the React frontend speaks.

## Why a separate Go module

This directory is its own module (`github.com/raysh454/moku/acceptance`) on
purpose: Go forbids importing another module's `internal/` packages, so the
suite **cannot compile** if a test reaches into moku's internals. The
black-box constraint is enforced by the compiler, not by code review.

If a behavior can't be tested from here, that's a missing product surface
(config flag, env var, API field) — add the surface, don't punch a hole in
the suite.

## Running

```bash
cd acceptance && go test ./... -v     # directly
go run make.go test-acceptance        # from the repo root
```

The first run builds `bin/acceptance-moku` and `bin/acceptance-demoserver`
from source; later scenarios in the same run reuse them. The whole suite runs
in a few seconds.

## Layout

| Package | Role |
|---|---|
| `scenarios/` | The executable specifications. Imports **only** `moku` and `demosite`. |
| `moku/` | Vocabulary for the system under test: `Start`, `GivenProject`, `GivenWebsite`, `WhenEnumerated`, `WhenFetched`, `WhenFetchStarted`, `Endpoint`, observations and `Then*` assertions. |
| `demosite/` | Vocabulary for the monitored target site (cmd/demoserver): `Start`, `URL`, `WhenAllPagesBumped`. |
| `internal/harness/` | Process plumbing: build-once, spawn, readiness, log capture, HTTP/SSE clients. Never import this from a scenario. |

## Writing a scenario

Specs are plain Go tests named as behaviors, structured Given/When/Then:

```go
func TestMonitoring_VersionBumpChangesAttackSurface(t *testing.T) {
	server := moku.Start(t)
	target := demosite.Start(t)

	project := server.GivenProject(t, "acme")
	site := project.GivenWebsite(t, target.URL())

	site.WhenEnumerated(t)
	site.WhenFetched(t)

	v1 := site.Endpoint("/").Snapshot(t)
	target.WhenAllPagesBumped(t)
	site.WhenFetched(t)

	v2 := site.Endpoint("/").WaitForNewVersion(t, v1)
	v2.ThenAttackSurfaceChangedSince(t, v1)
}
```

Ground rules:

- A scenario asserts **observable outcomes over HTTP**, never storage layout
  or internal state.
- Blocking verbs (`WhenFetched`) are the default; `WhenFetchStarted` returns a
  `Job` handle for scenarios about asynchronous behavior. Job completion is
  observed over the `/jobs/events` SSE stream, which keeps SSE permanently
  under test.
- If a spec needs something the vocabulary can't say, add a verb to `moku/` or
  `demosite/` — don't import the harness and don't hand-roll HTTP in the spec.
