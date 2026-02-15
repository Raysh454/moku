package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/server"
	"github.com/raysh454/moku/internal/testutil"
)

const (
	e2eHTTPTimeout  = 10 * time.Second
	e2eStartTimeout = 90 * time.Second
	e2ePollTimeout  = 90 * time.Second
	e2ePollEvery    = 250 * time.Millisecond
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type e2eClient struct {
	baseURL string
	http    *http.Client
}

func newE2EClient(baseURL string) *e2eClient {
	return &e2eClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: e2eHTTPTimeout,
		},
	}
}

func (c *e2eClient) request(method, path string, body any, out any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request body for %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body %s %s: %w", method, path, err)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, fmt.Errorf("decode response body %s %s: %w; body=%s", method, path, err, string(respBody))
		}
	}

	return resp.StatusCode, respBody, nil
}

func (c *e2eClient) mustStatus(t *testing.T, expected int, method, path string, body any, out any) []byte {
	t.Helper()
	code, respBody, err := c.request(method, path, body, out)
	if err != nil {
		t.Fatalf("%s %s request failed: %v", method, path, err)
	}
	if code != expected {
		t.Fatalf("%s %s expected status %d got %d; body=%s", method, path, expected, code, string(respBody))
	}
	return respBody
}

func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func repoRootFromThisFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func waitUntil(t *testing.T, timeout time.Duration, fn func() (bool, string, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastMsg string
	var lastErr error

	for time.Now().Before(deadline) {
		ok, msg, err := fn()
		if ok {
			return
		}
		lastMsg = msg
		lastErr = err
		time.Sleep(e2ePollEvery)
	}

	if lastErr != nil {
		t.Fatalf("timed out waiting: %s; last error: %v", lastMsg, lastErr)
	}
	t.Fatalf("timed out waiting: %s", lastMsg)
}

func startDemoServerProcess(t *testing.T, repoRoot string, port int) (string, *lockedBuffer) {
	t.Helper()

	output := &lockedBuffer{}
	binaryPath := filepath.Join(t.TempDir(), "demoserver-e2e")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/demoserver")
	buildCmd.Dir = repoRoot
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("build demo server binary: %v; output=%s", buildErr, string(buildOut))
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binaryPath, fmt.Sprintf("%d", port))
	cmd.Dir = repoRoot
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		t.Fatalf("start demo server process: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := newE2EClient(baseURL)

	waitUntil(t, e2eStartTimeout, func() (bool, string, error) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return false, "demo server exited before readiness", fmt.Errorf("demo output: %s", output.String())
		}

		code, body, err := client.request(http.MethodGet, "/demo/get-versions", nil, nil)
		if err != nil {
			return false, "waiting for demo server", err
		}
		if code == http.StatusOK {
			return true, "demo server ready", nil
		}
		return false, fmt.Sprintf("demo server not ready: status=%d body=%s", code, string(body)), nil
	})

	return baseURL, output
}

func startAPIServer(t *testing.T, port int) string {
	t.Helper()

	logger := &testutil.DummyLogger{}
	cfg := app.DefaultConfig()
	cfg.StorageRoot = t.TempDir()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	srv, err := server.NewServer(server.Config{
		ListenAddr: listenAddr,
		AppConfig:  cfg,
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("create api server: %v", err)
	}

	httpSrv := srv.HTTPServer()
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		srv.Close()

		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Logf("api server listen error during cleanup: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Logf("timeout waiting for api server to shut down")
		}
	})

	baseURL := fmt.Sprintf("http://%s", listenAddr)
	client := newE2EClient(baseURL)

	waitUntil(t, e2eStartTimeout, func() (bool, string, error) {
		code, body, err := client.request(http.MethodGet, "/projects", nil, nil)
		if err != nil {
			return false, "waiting for api server", err
		}
		if code == http.StatusOK {
			return true, "api server ready", nil
		}
		return false, fmt.Sprintf("api server not ready: status=%d body=%s", code, string(body)), nil
	})

	return baseURL
}

type demoVersionsEntry struct {
	Path           string `json:"path"`
	CurrentVersion int    `json:"current_version"`
}

type simpleSuccessResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type createProjectResp struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
}

type createWebsiteResp struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Origin string `json:"origin"`
}

type jobResp struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	Error          string   `json:"error"`
	EnumeratedURLs []string `json:"enumerated_urls"`
}

type endpointResp struct {
	CanonicalURL string `json:"canonical_url"`
	Status       string `json:"status"`
}

type snapshotResp struct {
	ID         string              `json:"id"`
	VersionID  string              `json:"version_id"`
	StatusCode int                 `json:"status_code"`
	URL        string              `json:"url"`
	Body       []byte              `json:"body"`
	Headers    map[string][]string `json:"headers"`
}

type bodyDiffResp struct {
	Chunks []struct {
		Type string `json:"type"`
	} `json:"chunks"`
}

type combinedDiffResp struct {
	BodyDiff bodyDiffResp `json:"body_diff"`
}

type securityDiffResp struct {
	BaseVersionID        string `json:"base_version_id"`
	HeadVersionID        string `json:"head_version_id"`
	AttackSurfaceChanged bool   `json:"attack_surface_changed"`
}

type endpointDetailsResp struct {
	Snapshot     *snapshotResp     `json:"snapshot"`
	SecurityDiff *securityDiffResp `json:"security_diff"`
	Diff         *combinedDiffResp `json:"diff"`
}

func waitForJobDone(t *testing.T, api *e2eClient, jobID string) jobResp {
	t.Helper()

	var final jobResp
	waitUntil(t, e2ePollTimeout, func() (bool, string, error) {
		var job jobResp
		api.mustStatus(t, http.StatusOK, http.MethodGet, "/jobs/"+jobID, nil, &job)

		switch job.Status {
		case "done":
			final = job
			return true, "job done", nil
		case "failed", "canceled":
			return false, fmt.Sprintf("job ended in %s: %s", job.Status, job.Error), errors.New("job did not complete successfully")
		default:
			return false, fmt.Sprintf("job status=%s", job.Status), nil
		}
	})

	return final
}

func findVersion(entries []demoVersionsEntry, path string) (int, bool) {
	for _, entry := range entries {
		if entry.Path == path {
			return entry.CurrentVersion, true
		}
	}
	return 0, false
}

func TestDemoE2E_HappyPath(t *testing.T) {
	repoRoot := repoRootFromThisFile(t)

	demoPort := getFreePort(t)
	apiPort := getFreePort(t)

	demoBaseURL, demoOutput := startDemoServerProcess(t, repoRoot, demoPort)
	apiBaseURL := startAPIServer(t, apiPort)

	demo := newE2EClient(demoBaseURL)
	api := newE2EClient(apiBaseURL)

	var resetResp simpleSuccessResp
	demo.mustStatus(t, http.StatusOK, http.MethodPost, "/demo/reset", nil, &resetResp)
	if !resetResp.Success {
		t.Fatalf("demo reset reported failure: %+v", resetResp)
	}

	var versionsBefore []demoVersionsEntry
	demo.mustStatus(t, http.StatusOK, http.MethodGet, "/demo/get-versions", nil, &versionsBefore)
	rootVBefore, ok := findVersion(versionsBefore, "/")
	if !ok {
		t.Fatalf("root path '/' not found in demo versions")
	}
	if rootVBefore != 1 {
		t.Fatalf("expected root version to be 1 after reset, got %d", rootVBefore)
	}

	var project createProjectResp
	api.mustStatus(t, http.StatusCreated, http.MethodPost, "/projects", map[string]any{
		"slug":        "demo-e2e",
		"name":        "Demo E2E",
		"description": "end-to-end demo test",
	}, &project)
	if project.ID == "" || project.Slug != "demo-e2e" {
		t.Fatalf("unexpected project response: %+v", project)
	}

	var website createWebsiteResp
	api.mustStatus(t, http.StatusCreated, http.MethodPost, "/projects/demo-e2e/websites", map[string]any{
		"slug":   "local-demo",
		"origin": demoBaseURL,
	}, &website)
	if website.ID == "" || website.Slug != "local-demo" || website.Origin != demoBaseURL {
		t.Fatalf("unexpected website response: %+v", website)
	}

	var enumerateJob jobResp
	api.mustStatus(t, http.StatusAccepted, http.MethodPost, "/projects/demo-e2e/websites/local-demo/jobs/enumerate", nil, &enumerateJob)
	if enumerateJob.ID == "" || enumerateJob.Type != "enumerate" {
		t.Fatalf("unexpected enumerate job response: %+v", enumerateJob)
	}

	enumerateDone := waitForJobDone(t, api, enumerateJob.ID)
	if len(enumerateDone.EnumeratedURLs) == 0 {
		t.Fatalf("enumeration produced no URLs")
	}

	var fetchJob1 jobResp
	api.mustStatus(t, http.StatusAccepted, http.MethodPost, "/projects/demo-e2e/websites/local-demo/jobs/fetch", map[string]any{
		"status": "*",
		"limit":  100,
	}, &fetchJob1)
	if fetchJob1.ID == "" || fetchJob1.Type != "fetch" {
		t.Fatalf("unexpected first fetch job response: %+v", fetchJob1)
	}
	_ = waitForJobDone(t, api, fetchJob1.ID)

	var endpoints []endpointResp
	api.mustStatus(t, http.StatusOK, http.MethodGet, "/projects/demo-e2e/websites/local-demo/endpoints?status=*&limit=200", nil, &endpoints)
	if len(endpoints) == 0 {
		t.Fatalf("endpoint list is empty after enumerate+fetch")
	}

	rootURL := demoBaseURL + "/"
	foundRoot := false
	for _, endpoint := range endpoints {
		if endpoint.CanonicalURL == rootURL {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Fatalf("root endpoint %q not found in indexed endpoints (count=%d)", rootURL, len(endpoints))
	}

	var detailsV1 endpointDetailsResp
	api.mustStatus(t, http.StatusOK, http.MethodGet, "/projects/demo-e2e/websites/local-demo/endpoints/details?url="+rootURL, nil, &detailsV1)
	if detailsV1.Snapshot == nil {
		t.Fatalf("details v1 snapshot is nil")
	}
	if detailsV1.Snapshot.StatusCode != http.StatusOK {
		t.Fatalf("details v1 unexpected status code: %d", detailsV1.Snapshot.StatusCode)
	}
	if detailsV1.Snapshot.VersionID == "" {
		t.Fatalf("details v1 snapshot version_id is empty")
	}
	bodyV1 := string(detailsV1.Snapshot.Body)
	if !strings.Contains(bodyV1, "Version 1 - Basic home page") {
		t.Fatalf("details v1 body does not contain expected marker; body=%s", bodyV1)
	}

	// Snapshot ordering uses second-level timestamps in storage.
	// Ensure the next fetch lands in a different second for deterministic "latest" reads.
	time.Sleep(1100 * time.Millisecond)

	var bumpResp simpleSuccessResp
	demo.mustStatus(t, http.StatusOK, http.MethodPost, "/demo/bump-all", nil, &bumpResp)
	if !bumpResp.Success {
		t.Fatalf("demo bump-all reported failure: %+v", bumpResp)
	}

	var versionsAfter []demoVersionsEntry
	demo.mustStatus(t, http.StatusOK, http.MethodGet, "/demo/get-versions", nil, &versionsAfter)
	rootVAfter, ok := findVersion(versionsAfter, "/")
	if !ok {
		t.Fatalf("root path '/' not found in demo versions after bump")
	}
	if rootVAfter != 2 {
		t.Fatalf("expected root version to be 2 after bump-all, got %d", rootVAfter)
	}

	var fetchJob2 jobResp
	api.mustStatus(t, http.StatusAccepted, http.MethodPost, "/projects/demo-e2e/websites/local-demo/jobs/fetch", map[string]any{
		"status": "*",
		"limit":  100,
	}, &fetchJob2)
	if fetchJob2.ID == "" || fetchJob2.Type != "fetch" {
		t.Fatalf("unexpected second fetch job response: %+v", fetchJob2)
	}
	_ = waitForJobDone(t, api, fetchJob2.ID)

	var detailsV2 endpointDetailsResp
	waitUntil(t, e2ePollTimeout, func() (bool, string, error) {
		api.mustStatus(t, http.StatusOK, http.MethodGet, "/projects/demo-e2e/websites/local-demo/endpoints/details?url="+rootURL, nil, &detailsV2)
		if detailsV2.Snapshot == nil {
			return false, "waiting for v2 snapshot details", nil
		}
		if detailsV2.Snapshot.VersionID == detailsV1.Snapshot.VersionID {
			return false, "waiting for snapshot version to change", nil
		}
		if !strings.Contains(string(detailsV2.Snapshot.Body), "Version 2 - Added admin and upload links") {
			return false, "waiting for v2 body marker", nil
		}
		return true, "v2 details ready", nil
	})
	if detailsV2.Snapshot == nil {
		t.Fatalf("details v2 snapshot is nil")
	}
	if detailsV2.Snapshot.VersionID == "" {
		t.Fatalf("details v2 snapshot version_id is empty")
	}
	if detailsV2.Snapshot.VersionID == detailsV1.Snapshot.VersionID {
		t.Fatalf("expected snapshot version to change after bump+fetch; still %s", detailsV2.Snapshot.VersionID)
	}

	bodyV2 := string(detailsV2.Snapshot.Body)
	if !strings.Contains(bodyV2, "Version 2 - Added admin and upload links") {
		t.Fatalf("details v2 body does not contain expected marker; body=%s", bodyV2)
	}
	if !strings.Contains(bodyV2, "id=\"search-form\"") {
		t.Fatalf("details v2 body missing expected added search form; body=%s", bodyV2)
	}

	if detailsV2.Diff == nil {
		t.Fatalf("expected non-nil diff after version bump")
	}
	if len(detailsV2.Diff.BodyDiff.Chunks) == 0 {
		t.Fatalf("expected body diff chunks after version bump")
	}

	if detailsV2.SecurityDiff == nil {
		t.Fatalf("expected non-nil security diff after version bump")
	}
	if detailsV2.SecurityDiff.BaseVersionID == "" || detailsV2.SecurityDiff.HeadVersionID == "" {
		t.Fatalf("security diff missing base/head version ids: %+v", detailsV2.SecurityDiff)
	}
	if detailsV2.SecurityDiff.BaseVersionID != detailsV1.Snapshot.VersionID {
		t.Fatalf("security diff base version %q does not match v1 version %q", detailsV2.SecurityDiff.BaseVersionID, detailsV1.Snapshot.VersionID)
	}
	if detailsV2.SecurityDiff.HeadVersionID != detailsV2.Snapshot.VersionID {
		t.Fatalf("security diff head version %q does not match v2 snapshot version %q", detailsV2.SecurityDiff.HeadVersionID, detailsV2.Snapshot.VersionID)
	}
	if !detailsV2.SecurityDiff.AttackSurfaceChanged {
		t.Fatalf("expected attack surface to change for root page from v1->v2")
	}

	if values, ok := detailsV2.Snapshot.Headers["x-content-type-options"]; !ok || len(values) == 0 || values[0] != "nosniff" {
		t.Fatalf("expected x-content-type-options=nosniff in v2 snapshot headers; headers=%v", detailsV2.Snapshot.Headers)
	}

	if strings.TrimSpace(demoOutput.String()) == "" {
		t.Logf("demo server output was empty")
	}
}
