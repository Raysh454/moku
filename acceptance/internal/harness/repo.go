// Package harness owns the process-level plumbing behind the acceptance DSL:
// building the real binaries, spawning them as subprocesses, and speaking
// plain HTTP to them. Scenario files must never import this package — they
// speak only the moku and demosite vocabularies.
package harness

import (
	"path/filepath"
	"runtime"
)

// RepoRoot returns the repository root, resolved from this file's location
// (acceptance/internal/harness -> three levels up). Resolution is static so
// the suite works regardless of the working directory go test was run from.
func RepoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("harness: cannot resolve repository root from caller information")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}
