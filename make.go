//go:build ignore

// make.go is a portable, dependency-free build script for moku.
// It replaces the Makefile for contributors who prefer (or are required to use)
// a toolchain that does not include `make` -- notably Windows users.
//
// Usage:
//
//	go run make.go <target> [args...]
//
// Run `go run make.go help` to see the full target list.
//
// Platform-independence contract:
//   - Never shell out (no bash, no cmd /c).
//   - Pass env vars via cmd.Env, not via shell prefixes.
//   - All paths built with filepath.Join.
//   - OS-conditional code lives only in the `exe` helper and the test-race
//     guard (the race detector requires a working CGO toolchain that is
//     not commonly available on Windows -- matches the original Makefile).
//   - Every tool version is pinned -- no @latest anywhere.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	binaryName     = "moku"
	demoServerName = "demo-server"
	swaggerEntry   = "internal/server/server.go"
	swaggerOutDir  = "docs/swagger"

	golangciLintVersion = "v1.64.8"
	swagVersion         = "v1.16.4"

	golangciLintPkg = "github.com/golangci/golangci-lint/cmd/golangci-lint"
	swagPkg         = "github.com/swaggo/swag/cmd/swag"
)

var isWindows = runtime.GOOS == "windows"

func exe(name string) string {
	if isWindows {
		return name + ".exe"
	}
	return name
}

func binDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	return filepath.Join(cwd, "bin")
}

type targetFunc func(args []string) error

type target struct {
	run  targetFunc
	help string
}

func targets() map[string]target {
	return map[string]target{
		"build":            {build, "Build the moku binary into bin/ (regenerates swagger docs first)"},
		"run":              {run, "Build and run the moku binary; remaining args forwarded"},
		"demo-server":      {demoServer, "Build the demo-server binary into bin/"},
		"test":             {test, "Run all tests with verbose output"},
		"test-race":        {testRace, "Run all tests with the race detector (skipped on Windows)"},
		"test-pkg":         {testPkg, "Run tests for a single package: test-pkg <pkg-path>"},
		"fmt":              {fmtCmd, "Format all Go source files with gofmt -w"},
		"vet":              {vet, "Run go vet ./..."},
		"lint":             {lint, "Run golangci-lint (installs to bin/ if missing)"},
		"install-golangci": {installGolangci, "Install golangci-lint " + golangciLintVersion + " into bin/"},
		"install-swagger":  {installSwagger, "Install swag " + swagVersion + " into bin/"},
		"swagger":          {swagger, "Regenerate Swagger docs under docs/swagger/"},
		"coverage":         {coverage, "Run tests with coverage and write test-results/coverage.txt"},
		"coverage-html":    {coverageHTML, "Produce coverage.html from coverage.out"},
		"ci":               {ci, "Run the full CI pipeline (swagger, fmt, vet, lint, test-race, coverage)"},
		"clean":            {clean, "Remove bin/, coverage.out, coverage.html, test-results/"},
		"help":             {help, "Print this help message"},
	}
}

func main() {
	if len(os.Args) < 2 {
		_ = help(nil)
		os.Exit(1)
	}
	name := os.Args[1]
	args := os.Args[2:]

	t, ok := targets()[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown target: %s\n\n", name)
		_ = help(nil)
		os.Exit(1)
	}
	if err := t.run(args); err != nil {
		fmt.Fprintf(os.Stderr, "==> %s failed: %v\n", name, err)
		os.Exit(1)
	}
}

// --- helpers -----------------------------------------------------------------

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "fatal:", err)
	os.Exit(1)
}

func info(format string, a ...any) {
	fmt.Printf("==> "+format+"\n", a...)
}

// runCmd executes name with args, streaming stdout/stderr to this process.
func runCmd(name string, args ...string) error {
	return runCmdWith(nil, name, args...)
}

// runCmdWith executes name with args and extra environment variables (KEY=VALUE).
// extraEnv is appended to os.Environ() so the child sees everything the parent
// sees, plus the additions/overrides. Same behavior on Linux and Windows.
func runCmdWith(extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Run()
}

// runCmdCapture runs name with args and returns stdout (with stderr streamed
// to the parent process for visibility).
func runCmdCapture(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func removeAll(paths ...string) error {
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return nil
}

// --- targets -----------------------------------------------------------------

func help(_ []string) error {
	w := os.Stdout
	fmt.Fprintln(w, "moku build script -- portable replacement for Makefile.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  go run make.go <target> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Targets:")
	all := targets()
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)
	width := 0
	for _, n := range names {
		if len(n) > width {
			width = len(n)
		}
	}
	for _, n := range names {
		fmt.Fprintf(w, "  %-*s   %s\n", width, n, all[n].help)
	}
	return nil
}

func build(_ []string) error {
	if err := installSwagger(nil); err != nil {
		return err
	}
	if err := swagger(nil); err != nil {
		return err
	}
	if err := ensureDir(binDir()); err != nil {
		return err
	}
	out := filepath.Join(binDir(), exe(binaryName))
	info("building root package -> %s", out)
	return runCmd("go", "build", "-v", "-o", out, ".")
}

func run(args []string) error {
	if err := build(nil); err != nil {
		return err
	}
	bin := filepath.Join(binDir(), exe(binaryName))
	info("running %s", bin)
	return runCmd(bin, args...)
}

func demoServer(_ []string) error {
	if err := ensureDir(binDir()); err != nil {
		return err
	}
	out := filepath.Join(binDir(), exe(demoServerName))
	info("building demo-server -> %s", out)
	return runCmd("go", "build", "-v", "-o", out, "./cmd/demoserver")
}

func test(_ []string) error {
	info("go test ./...")
	return runCmd("go", "test", "./...", "-v")
}

func testRace(_ []string) error {
	if isWindows {
		info("race detector is not supported on Windows; skipping")
		return nil
	}
	info("go test -race ./... (with SKIP_CHROMEDP=1)")
	return runCmdWith([]string{"SKIP_CHROMEDP=1"}, "go", "test", "-race", "./...", "-v")
}

func testPkg(args []string) error {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return errors.New("test-pkg requires a package argument, e.g. test-pkg ./internal/webclient")
	}
	pkg := args[0]
	info("go test %s", pkg)
	return runCmd("go", "test", pkg, "-v")
}

func fmtCmd(_ []string) error {
	info("gofmt -l -w .")
	return runCmd("gofmt", "-l", "-w", ".")
}

func vet(_ []string) error {
	info("go vet ./...")
	return runCmd("go", "vet", "./...")
}

func lint(_ []string) error {
	if err := installGolangci(nil); err != nil {
		return err
	}
	bin := filepath.Join(binDir(), exe("golangci-lint"))
	info("golangci-lint run")
	return runCmd(bin, "run")
}

func installGolangci(_ []string) error {
	bin := filepath.Join(binDir(), exe("golangci-lint"))
	if fileExists(bin) {
		return nil
	}
	if err := ensureDir(binDir()); err != nil {
		return err
	}
	info("installing golangci-lint %s -> %s", golangciLintVersion, bin)
	return runCmdWith(
		[]string{"GOBIN=" + binDir()},
		"go", "install", golangciLintPkg+"@"+golangciLintVersion,
	)
}

func installSwagger(_ []string) error {
	bin := filepath.Join(binDir(), exe("swag"))
	if fileExists(bin) {
		return nil
	}
	if err := ensureDir(binDir()); err != nil {
		return err
	}
	info("installing swag %s -> %s", swagVersion, bin)
	return runCmdWith(
		[]string{"GOBIN=" + binDir()},
		"go", "install", swagPkg+"@"+swagVersion,
	)
}

func swagger(_ []string) error {
	if err := installSwagger(nil); err != nil {
		return err
	}
	bin := filepath.Join(binDir(), exe("swag"))
	info("generating Swagger docs -> %s", swaggerOutDir)
	return runCmd(bin, "init", "-g", swaggerEntry, "-o", swaggerOutDir)
}

func coverage(_ []string) error {
	if err := ensureDir("test-results"); err != nil {
		return err
	}
	info("running tests with coverage (SKIP_CHROMEDP=1)")
	err := runCmdWith(
		[]string{"SKIP_CHROMEDP=1"},
		"go", "test", "./...", "-coverprofile=coverage.out", "-covermode=atomic", "-v",
	)
	if err != nil {
		return err
	}
	info("coverage summary")
	out, err := runCmdCapture("go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("test-results", "coverage.txt"), out, 0o644); err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func coverageHTML(_ []string) error {
	if !fileExists("coverage.out") {
		return errors.New("coverage.out not found; run `go run make.go coverage` first")
	}
	info("generating coverage HTML")
	if err := runCmd("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html"); err != nil {
		return err
	}
	info("coverage.html generated")
	return nil
}

func ci(_ []string) error {
	steps := []struct {
		name string
		fn   targetFunc
	}{
		{"install-swagger", installSwagger},
		{"swagger", swagger},
		{"fmt", fmtCmd},
		{"vet", vet},
		{"install-golangci", installGolangci},
		{"lint", lint},
		{"test-race", testRace},
		{"coverage", coverage},
	}
	for _, s := range steps {
		info("ci step: %s", s.name)
		if err := s.fn(nil); err != nil {
			return fmt.Errorf("ci step %s: %w", s.name, err)
		}
	}
	info("CI checks completed")
	return nil
}

func clean(_ []string) error {
	info("cleaning")
	return removeAll(binDir(), "coverage.out", "coverage.html", "test-results")
}
