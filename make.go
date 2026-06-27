//go:build ignore

// make.go is Moku's portable, dependency-free build script.
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
	swagVersion         = "v1.16.6"

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

func absPath(rel ...string) string {
	abs, err := filepath.Abs(filepath.Join(rel...))
	if err != nil {
		fatal(err)
	}
	return abs
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
		"run-with-sidecar": {runWithSidecar, "Build and run the moku binary with the sidecar service running"},
		"demo-server":      {demoServer, "Build the demo-server binary into bin/"},
		"test":             {test, "Run all tests with verbose output"},
		"test-race":        {testRace, "Run all tests with the race detector (skipped on Windows)"},
		"test-pkg":         {testPkg, "Run tests for a single package: test-pkg <pkg-path>"},
		"test-acceptance":  {testAcceptance, "Run acceptance tests"},
		"fmt":              {fmtCmd, "Format all Go source files with gofmt -w"},
		"vet":              {vet, "Run go vet ./..."},
		"lint":             {lint, "Run golangci-lint (installs to bin/ if missing)"},
		"install-golangci": {installGolangci, "Install golangci-lint " + golangciLintVersion + " into bin/"},
		"install-swagger":  {installSwagger, "Install swag " + swagVersion + " into bin/"},
		"install-browser":  {installBrowser, "Install chrome-headless-shell into ~/.cache/puppeteer and print MOKU_CHROME_PATH"},
		"swagger":          {swagger, "Regenerate Swagger docs under docs/swagger/"},
		"coverage":         {coverage, "Run tests with coverage and write test-results/coverage.txt"},
		"coverage-html":    {coverageHTML, "Produce coverage.html from coverage.out"},
		"ci":               {ci, "Run the full CI pipeline (swagger, fmt, vet, lint, test-race, coverage, acceptance, sidecar)"},
		"clean":            {clean, "Remove bin/, coverage.out, coverage.html, test-results/"},
		"sidecar-install":  {sidecarInstall, "Install sidecar Python dependencies"},
		"sidecar-start":    {sidecarStart, "Start the sidecar service"},
		"sidecar-stop":     {sidecarStop, "Stop the sidecar service"},
		"sidecar-health":   {sidecarHealth, "Check the health of the sidecar service"},
		"sidecar-test":     {sidecarTest, "Run sidecar pytest suite"},
		"sidecar-clean":    {sidecarClean, "Clean sidecar runtime artifacts"},
		"schema-check":     {schemaCheck, "Validate sidecar JSON fixtures against ScanResult schema"},
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

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func checkPython() (string, bool) {
	if p, err := exec.LookPath("python"); err == nil {
		return p, true
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p, true
	}
	return "", false
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

func runWithSidecar(args []string) error {
	if err := sidecarStart(nil); err != nil {
		return err
	}
	if err := build(nil); err != nil {
		return err
	}
	bin := filepath.Join(binDir(), exe(binaryName))
	info("running %s with sidecar", bin)
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

func testAcceptance(_ []string) error {
	info("acceptance suite (acceptance/)")
	cmd := exec.Command("go", "test", "./...", "-v")
	cmd.Dir = absPath("acceptance")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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
	bin := filepath.Join(binDir(), exe("golangci-lint-"+golangciLintVersion))
	info("golangci-lint run (%s)", golangciLintVersion)
	return runCmd(bin, "run")
}

func installGolangci(_ []string) error {
	versionedBin := filepath.Join(binDir(), exe("golangci-lint-"+golangciLintVersion))
	if fileExists(versionedBin) {
		return nil
	}
	if err := ensureDir(binDir()); err != nil {
		return err
	}
	info("installing golangci-lint %s -> %s", golangciLintVersion, versionedBin)
	err := runCmdWith(
		[]string{"GOBIN=" + binDir()},
		"go", "install", golangciLintPkg+"@"+golangciLintVersion,
	)
	if err != nil {
		return err
	}
	src := filepath.Join(binDir(), exe("golangci-lint"))
	return copyFile(src, versionedBin)
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

// installBrowser installs chrome-headless-shell into the per-user puppeteer
// cache (never into the repo) via @puppeteer/browsers, then prints the
// MOKU_CHROME_PATH the chromedp backend should use. The browser is a
// machine-level runtime dependency, so it is kept out of the working tree.
func installBrowser(_ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locating home dir: %w", err)
	}
	cacheDir := filepath.Join(home, ".cache", "puppeteer")
	if err := ensureDir(cacheDir); err != nil {
		return err
	}

	info("installing chrome-headless-shell -> %s", cacheDir)
	if err := runCmd("npx", "--yes", "@puppeteer/browsers", "install", "chrome-headless-shell@stable", "--path", cacheDir); err != nil {
		return fmt.Errorf("installing chrome-headless-shell via npx (is Node.js/npm installed?): %w", err)
	}

	binary, err := findHeadlessShell(cacheDir)
	if err != nil {
		info("installed under %s, but could not locate the binary automatically (%v); see the output above", cacheDir, err)
		return nil
	}
	info("chrome-headless-shell ready: %s", binary)
	info("point moku at it via MOKU_CHROME_PATH:")
	if isWindows {
		info(`  setx MOKU_CHROME_PATH "%s"`, binary)
	} else {
		info(`  export MOKU_CHROME_PATH="%s"`, binary)
	}
	return nil
}

// findHeadlessShell locates the chrome-headless-shell executable under the
// puppeteer cache, preferring the highest version directory.
func findHeadlessShell(cacheDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(cacheDir, "chrome-headless-shell", "*", "*", exe("chrome-headless-shell")))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no chrome-headless-shell binary under %s", cacheDir)
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
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
		{"test-acceptance", testAcceptance},
	}
	for _, s := range steps {
		info("ci step: %s", s.name)
		if err := s.fn(nil); err != nil {
			return fmt.Errorf("ci step %s: %w", s.name, err)
		}
	}
	if _, hasPy := checkPython(); hasPy {
		info("ci step: sidecar-test")
		if err := sidecarTest(nil); err != nil {
			return fmt.Errorf("ci step sidecar-test: %w", err)
		}
	} else {
		info("python not found; skipping sidecar tests")
	}
	info("CI checks completed")
	return nil
}

func clean(_ []string) error {
	info("cleaning")
	return removeAll(binDir(), "coverage.out", "coverage.html", "test-results")
}

func sidecarInstall(_ []string) error {
	installedFile := absPath("services", "analyzer", ".installed")
	reqsFile := absPath("services", "analyzer", "requirements.txt")
	reqsDevFile := absPath("services", "analyzer", "requirements-dev.txt")

	venvPython := absPath("services", "analyzer", ".venv", "bin", "python")
	if isWindows {
		venvPython = absPath("services", "analyzer", ".venv", "Scripts", "python.exe")
	}

	needInstall := true
	if fileExists(installedFile) && fileExists(venvPython) {
		instStat, err1 := os.Stat(installedFile)
		reqsStat, err2 := os.Stat(reqsFile)
		reqsDevStat, err3 := os.Stat(reqsDevFile)
		if err1 == nil && err2 == nil && err3 == nil {
			if instStat.ModTime().After(reqsStat.ModTime()) && instStat.ModTime().After(reqsDevStat.ModTime()) {
				needInstall = false
			}
		}
	}

	if !needInstall {
		return nil
	}

	info("installing sidecar dependencies")

	if !fileExists(venvPython) {
		pyCmd, hasPy := checkPython()
		if !hasPy {
			return errors.New("python/python3 not found; cannot create virtual environment")
		}
		venvPath := absPath("services", "analyzer", ".venv")
		if err := runCmd(pyCmd, "-m", "venv", venvPath); err != nil {
			return fmt.Errorf("create venv: %w", err)
		}
	}

	if err := runCmd(venvPython, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return fmt.Errorf("upgrade pip: %w", err)
	}

	if err := runCmd(venvPython, "-m", "pip", "install", "-r", reqsFile, "-r", reqsDevFile); err != nil {
		return fmt.Errorf("pip install: %w", err)
	}

	return os.WriteFile(installedFile, []byte{}, 0o644)
}

func sidecarStart(_ []string) error {
	info("starting sidecar")
	if isWindows {
		return runCmd("pwsh", "-ExecutionPolicy", "Bypass", "-File", absPath("services", "analyzer", "scripts", "start.ps1"))
	}
	return runCmd("bash", absPath("services", "analyzer", "scripts", "start.sh"))
}

func sidecarStop(_ []string) error {
	info("stopping sidecar")
	if isWindows {
		return runCmd("pwsh", "-ExecutionPolicy", "Bypass", "-File", absPath("services", "analyzer", "scripts", "stop.ps1"))
	}
	return runCmd("bash", absPath("services", "analyzer", "scripts", "stop.sh"))
}

func sidecarHealth(_ []string) error {
	info("checking sidecar health")
	if isWindows {
		return runCmd("pwsh", "-ExecutionPolicy", "Bypass", "-File", absPath("services", "analyzer", "scripts", "health.ps1"))
	}
	return runCmd("bash", absPath("services", "analyzer", "scripts", "health.sh"))
}

func sidecarTest(_ []string) error {
	if err := sidecarInstall(nil); err != nil {
		return err
	}
	info("running sidecar pytest suite")
	venvPython := absPath("services", "analyzer", ".venv", "bin", "python")
	if isWindows {
		venvPython = absPath("services", "analyzer", ".venv", "Scripts", "python.exe")
	}
	cmd := exec.Command(venvPython, "-m", "pytest", "tests/", "-v")
	cmd.Dir = absPath("services", "analyzer")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func sidecarClean(_ []string) error {
	info("cleaning sidecar runtime artifacts")
	paths := []string{
		absPath("services", "analyzer", ".venv"),
		absPath("services", "analyzer", ".run"),
		absPath("services", "analyzer", ".pytest_cache"),
		absPath("services", "analyzer", ".installed"),
	}
	if err := removeAll(paths...); err != nil {
		return err
	}
	return filepath.Walk(absPath("services", "analyzer"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == "__pycache__" {
			_ = os.RemoveAll(path)
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".db") {
			_ = os.Remove(path)
		}
		return nil
	})
}

func schemaCheck(_ []string) error {
	if err := sidecarInstall(nil); err != nil {
		return err
	}
	info("validating sidecar JSON fixtures against ScanResult schema")
	venvPython := absPath("services", "analyzer", ".venv", "bin", "python")
	if isWindows {
		venvPython = absPath("services", "analyzer", ".venv", "Scripts", "python.exe")
	}
	cmd := exec.Command(venvPython, "scripts/schema_check.py")
	cmd.Dir = absPath("services", "analyzer")
	cmd.Env = append(os.Environ(), "PYTHONPATH=.")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
