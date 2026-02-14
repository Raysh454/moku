package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/server"
	"github.com/raysh454/moku/internal/testutil"
)

func newTestServer(t *testing.T) *server.Server {
	t.Helper()

	dir := t.TempDir()
	logger := &testutil.DummyLogger{}
	cfg := server.Config{
		ListenAddr: ":0",
		AppConfig: &app.Config{
			StorageRoot: dir,
		},
		Logger: logger,
	}
	// Override defaults that matter
	if cfg.AppConfig.JobRetentionTime == 0 {
		appCfg := app.DefaultConfig()
		appCfg.StorageRoot = dir
		cfg.AppConfig = appCfg
	}

	s, err := server.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func doJSON(t *testing.T, s http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON response: %v (body: %s)", err, rec.Body.String())
	}
}

// ─── CORS ──────────────────────────────────────────────────────────────

func TestServer_CORS_HeaderPresent(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "GET", "/projects", "")

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected CORS origin *, got %q", origin)
	}
}

// ─── Projects ──────────────────────────────────────────────────────────

func TestServer_CreateProject(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "POST", "/projects", `{"slug":"myproj","name":"My Project","description":"desc"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var p map[string]any
	decodeJSON(t, rec, &p)
	if p["slug"] != "myproj" {
		t.Errorf("expected slug 'myproj', got %v", p["slug"])
	}
}

func TestServer_CreateProject_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "POST", "/projects", `{invalid}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestServer_ListProjects_Empty(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "GET", "/projects", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServer_ListProjects_AfterCreate(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"p1","name":"P1"}`)

	rec := doJSON(t, s, "GET", "/projects", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var projects []map[string]any
	decodeJSON(t, rec, &projects)
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

// ─── Websites ──────────────────────────────────────────────────────────

func TestServer_CreateWebsite(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)

	rec := doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var web map[string]any
	decodeJSON(t, rec, &web)
	if web["origin"] != "https://example.com" {
		t.Errorf("unexpected origin: %v", web["origin"])
	}
}

func TestServer_CreateWebsite_InvalidJSON(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)

	rec := doJSON(t, s, "POST", "/projects/proj/websites", `not-json`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestServer_ListWebsites(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	rec := doJSON(t, s, "GET", "/projects/proj/websites", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var ws []map[string]any
	decodeJSON(t, rec, &ws)
	if len(ws) != 1 {
		t.Errorf("expected 1 website, got %d", len(ws))
	}
}

// ─── Jobs ──────────────────────────────────────────────────────────────

func TestServer_ListJobs_Empty(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "GET", "/jobs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestServer_GetJob_NotFound(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "GET", "/jobs/nonexistent", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestServer_CancelJob_NoContent(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "DELETE", "/jobs/nonexistent", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

// ─── Endpoints ─────────────────────────────────────────────────────────

func TestServer_GetEndpointDetails_MissingURL(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	rec := doJSON(t, s, "GET", "/projects/proj/websites/site/endpoints/details", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing url param, got %d", rec.Code)
	}
}

// ─── Options preflight ─────────────────────────────────────────────────

func TestServer_OptionsPreflight(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "OPTIONS", "/projects", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Allow-Methods header on OPTIONS")
	}
}
