package harness

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// buildResults caches one build per binary name for the lifetime of the test
// process, so every scenario can ask for the binary and only the first pays.
var (
	buildMu      sync.Mutex
	buildResults = map[string]buildResult{}
)

type buildResult struct {
	path string
	err  error
}

// BuildBinary compiles the Go package at pkgPath (relative to the repository
// root) into bin/<name> under the repository root and returns the binary
// path. bin/ is gitignored, so repeated runs just overwrite the previous
// build.
func BuildBinary(t *testing.T, pkgPath, name string) string {
	t.Helper()

	buildMu.Lock()
	defer buildMu.Unlock()

	if cached, ok := buildResults[name]; ok {
		failOnBuildError(t, name, cached.err)
		return cached.path
	}

	result := compile(pkgPath, name)
	buildResults[name] = result
	failOnBuildError(t, name, result.err)
	return result.path
}

func compile(pkgPath, name string) buildResult {
	root := RepoRoot()
	outPath := filepath.Join(root, "bin", binaryFileName(name))

	cmd := exec.Command("go", "build", "-o", outPath, "./"+filepath.ToSlash(pkgPath))
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		return buildResult{err: fmt.Errorf("go build %s: %w; output:\n%s", pkgPath, err, output)}
	}
	return buildResult{path: outPath}
}

func binaryFileName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func failOnBuildError(t *testing.T, name string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("harness: build %s failed: %v", name, err)
	}
}
