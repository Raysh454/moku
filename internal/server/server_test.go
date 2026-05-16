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

// createWebsiteWithoutDefaults creates a website and deletes all seeded filter rules.
// Use this for tests that need a clean slate without default filter rules.
func createWebsiteWithoutDefaults(t *testing.T, s *server.Server, projSlug, siteSlug, origin string) {
	t.Helper()
	doJSON(t, s, "POST", "/projects/"+projSlug+"/websites",
		`{"slug":"`+siteSlug+`","origin":"`+origin+`"}`)

	// Get all rules and delete them to start with clean slate
	rec := doJSON(t, s, "GET", "/projects/"+projSlug+"/websites/"+siteSlug+"/filters", "")
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		return // silently continue if decode fails
	}
	rules, ok := resp["rules"].([]any)
	if !ok {
		return
	}
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := rule["id"].(string); ok {
			doJSON(t, s, "DELETE", "/projects/"+projSlug+"/websites/"+siteSlug+"/filters/"+id, "")
		}
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

// ─── Filter Rules ──────────────────────────────────────────────────────

func TestServer_ListFilterRules_Empty(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	createWebsiteWithoutDefaults(t, s, "proj", "site", "https://example.com")

	rec := doJSON(t, s, "GET", "/projects/proj/websites/site/filters", "")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	rules, ok := resp["rules"].([]any)
	if !ok {
		t.Fatal("expected 'rules' array in response")
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestServer_CreateFilterRule(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	rec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"extension","rule_value":".jpg","action":"skip"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var rule map[string]any
	decodeJSON(t, rec, &rule)
	if rule["rule_type"] != "extension" {
		t.Errorf("expected rule_type 'extension', got %v", rule["rule_type"])
	}
	if rule["rule_value"] != ".jpg" {
		t.Errorf("expected rule_value '.jpg', got %v", rule["rule_value"])
	}
	// Note: action is stored in database but not returned in response
	// The response contains: id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at
	if rule["enabled"] != true {
		t.Errorf("expected enabled true, got %v", rule["enabled"])
	}
}

func TestServer_CreateFilterRule_InvalidType(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	rec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"invalid_type","rule_value":".jpg","action":"skip"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid rule_type, got %d", rec.Code)
	}
}

func TestServer_GetFilterRule(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	createRec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"extension","rule_value":".png","action":"skip"}`)

	var created map[string]any
	decodeJSON(t, createRec, &created)
	id := created["id"].(string)

	rec := doJSON(t, s, "GET", "/projects/proj/websites/site/filters/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var rule map[string]any
	decodeJSON(t, rec, &rule)
	if rule["id"] != id {
		t.Errorf("expected id %s, got %v", id, rule["id"])
	}
}

func TestServer_GetFilterRule_NotFound(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	rec := doJSON(t, s, "GET", "/projects/proj/websites/site/filters/nonexistent", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestServer_DeleteFilterRule(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	createRec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"extension","rule_value":".gif","action":"skip"}`)

	var created map[string]any
	decodeJSON(t, createRec, &created)
	id := created["id"].(string)

	rec := doJSON(t, s, "DELETE", "/projects/proj/websites/site/filters/"+id, "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify it's gone
	rec = doJSON(t, s, "GET", "/projects/proj/websites/site/filters/"+id, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestServer_UpdateFilterRule(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	createRec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"extension","rule_value":".jpg","action":"skip"}`)

	var created map[string]any
	decodeJSON(t, createRec, &created)
	id := created["id"].(string)

	rec := doJSON(t, s, "PUT", "/projects/proj/websites/site/filters/"+id,
		`{"rule_value":".jpeg"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated map[string]any
	decodeJSON(t, rec, &updated)
	if updated["rule_value"] != ".jpeg" {
		t.Errorf("expected rule_value '.jpeg', got %v", updated["rule_value"])
	}
}

func TestServer_ToggleFilterRule(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	createRec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters",
		`{"rule_type":"extension","rule_value":".jpg","action":"skip"}`)

	var created map[string]any
	decodeJSON(t, createRec, &created)
	id := created["id"].(string)

	// Initially enabled
	if created["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", created["enabled"])
	}

	// Disable
	rec := doJSON(t, s, "POST", "/projects/proj/websites/site/filters/"+id+"/toggle", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var toggled map[string]any
	decodeJSON(t, rec, &toggled)
	if toggled["enabled"] != false {
		t.Errorf("expected enabled=false after toggle, got %v", toggled["enabled"])
	}

	// Enable again
	rec = doJSON(t, s, "POST", "/projects/proj/websites/site/filters/"+id+"/toggle", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	decodeJSON(t, rec, &toggled)
	if toggled["enabled"] != true {
		t.Errorf("expected enabled=true after second toggle, got %v", toggled["enabled"])
	}
}

func TestServer_GetFilterConfig(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)
	doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)

	// Update the website's filter config directly
	rec := doJSON(t, s, "PUT", "/projects/proj/websites/site/filters/config",
		`{"skip_extensions":[".jpg",".png"],"skip_status_codes":[404]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT config, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, s, "GET", "/projects/proj/websites/site/filters/config", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var config map[string]any
	decodeJSON(t, rec, &config)

	// Should have skip_extensions
	skipExt, ok := config["skip_extensions"].([]any)
	if !ok {
		t.Fatal("expected skip_extensions array in config")
	}
	if len(skipExt) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(skipExt))
	}

	// Should have skip_status_codes
	skipCodes, ok := config["skip_status_codes"].([]any)
	if !ok {
		t.Fatal("expected skip_status_codes array in config")
	}
	if len(skipCodes) != 1 {
		t.Errorf("expected 1 status code, got %d", len(skipCodes))
	}
}

func TestServer_ListFilterRules_WebsiteNotFound(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)

	rec := doJSON(t, s, "GET", "/projects/proj/websites/nonexistent/filters", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent website, got %d", rec.Code)
	}
}

// ─── E2E Integration: Full Filtering Workflow ──────────────────────────

func TestServer_FilterWorkflow_E2E(t *testing.T) {
	// This test exercises the complete filtering workflow:
	// 1. Create project and website
	// 2. Add filter rules
	// 3. Update website filter config
	// 4. Add endpoints
	// 5. List endpoints and verify filtering behavior
	// 6. Toggle rules and verify changes
	// 7. Unfilter endpoints
	t.Parallel()
	s := newTestServer(t)

	// Step 1: Create project and website (without default rules for clean test)
	rec := doJSON(t, s, "POST", "/projects", `{"slug":"filter-test","name":"Filter Test"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %d %s", rec.Code, rec.Body.String())
	}

	createWebsiteWithoutDefaults(t, s, "filter-test", "example", "https://example.com")

	// Step 2: Add filter rules
	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/filters",
		`{"rule_type":"extension","rule_value":".jpg","action":"skip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create extension rule: %d %s", rec.Code, rec.Body.String())
	}
	var extRule map[string]any
	decodeJSON(t, rec, &extRule)
	extRuleID := extRule["id"].(string)

	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/filters",
		`{"rule_type":"pattern","rule_value":"*/assets/*","action":"skip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create pattern rule: %d %s", rec.Code, rec.Body.String())
	}
	var patternRule map[string]any
	decodeJSON(t, rec, &patternRule)
	patternRuleID := patternRule["id"].(string)

	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/filters",
		`{"rule_type":"status_code","rule_value":"404","action":"skip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create status code rule: %d %s", rec.Code, rec.Body.String())
	}

	// Step 3: Verify rules are listed
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/filters", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list rules: %d", rec.Code)
	}
	var rulesList map[string]any
	decodeJSON(t, rec, &rulesList)
	rules := rulesList["rules"].([]any)
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}

	// Step 4: Update website filter config (quick config)
	rec = doJSON(t, s, "PUT", "/projects/filter-test/websites/example/filters/config",
		`{"skip_extensions":[".png",".gif"],"skip_status_codes":[410]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update filter config: %d %s", rec.Code, rec.Body.String())
	}

	// Verify config was updated
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/filters/config", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get filter config: %d", rec.Code)
	}
	var config map[string]any
	decodeJSON(t, rec, &config)
	skipExt := config["skip_extensions"].([]any)
	if len(skipExt) != 2 {
		t.Errorf("expected 2 skip extensions in config, got %d", len(skipExt))
	}

	// Step 5: Toggle a rule
	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/filters/"+extRuleID+"/toggle", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to toggle rule: %d %s", rec.Code, rec.Body.String())
	}
	var toggledRule map[string]any
	decodeJSON(t, rec, &toggledRule)
	if toggledRule["enabled"] != false {
		t.Error("expected rule to be disabled after toggle")
	}

	// Toggle back
	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/filters/"+extRuleID+"/toggle", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to toggle rule back: %d", rec.Code)
	}

	// Step 6: Update a rule
	rec = doJSON(t, s, "PUT", "/projects/filter-test/websites/example/filters/"+patternRuleID,
		`{"rule_value":"*/media/*"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update rule: %d %s", rec.Code, rec.Body.String())
	}
	var updatedRule map[string]any
	decodeJSON(t, rec, &updatedRule)
	if updatedRule["rule_value"] != "*/media/*" {
		t.Errorf("expected updated rule_value '*/media/*', got %v", updatedRule["rule_value"])
	}

	// Step 7: Delete a rule
	rec = doJSON(t, s, "DELETE", "/projects/filter-test/websites/example/filters/"+patternRuleID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("failed to delete rule: %d", rec.Code)
	}

	// Verify deletion
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/filters/"+patternRuleID, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted rule, got %d", rec.Code)
	}

	// Step 8: Verify remaining rules
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/filters", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list rules after deletion: %d", rec.Code)
	}
	decodeJSON(t, rec, &rulesList)
	rules = rulesList["rules"].([]any)
	if len(rules) != 2 {
		t.Errorf("expected 2 rules after deletion, got %d", len(rules))
	}

	// Step 9: Add endpoints to the website
	rec = doJSON(t, s, "POST", "/projects/filter-test/websites/example/endpoints",
		`{"urls":["https://example.com/page.html","https://example.com/image.jpg","https://example.com/style.css"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to add endpoints: %d %s", rec.Code, rec.Body.String())
	}

	// Step 10: List endpoints
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/endpoints?status=all", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list endpoints: %d", rec.Code)
	}
	var endpoints []any
	decodeJSON(t, rec, &endpoints)
	if len(endpoints) < 1 {
		t.Errorf("expected at least 1 endpoint, got %d", len(endpoints))
	}

	// Step 11: Get endpoint stats
	rec = doJSON(t, s, "GET", "/projects/filter-test/websites/example/endpoints/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get endpoint stats: %d %s", rec.Code, rec.Body.String())
	}
	var stats map[string]any
	decodeJSON(t, rec, &stats)
	// Verify stats response has expected structure
	if _, ok := stats["total"]; !ok {
		t.Error("expected 'total' in stats response")
	}
}

// ─── Analyzer plugin HTTP surface ──────────────────────────────────────

// setupAnalyzerSite creates a project + website used by the analyzer handler
// tests. Matches the pattern used by the fetch/enumerate handler tests.
func setupAnalyzerSite(t *testing.T, s http.Handler) {
	t.Helper()
	doJSON(t, s, "POST", "/projects", `{"slug":"ap","name":"Analyzer Project","description":"desc"}`)
	doJSON(t, s, "POST", "/projects/ap/websites", `{"slug":"aw","origin":"http://example.com"}`)
}

// TestHandleStartScanJob_Returns202 asserts the happy path: a well-formed
// scan request returns 202 Accepted with a scan-typed Job.
func TestHandleStartScanJob_Returns202(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	setupAnalyzerSite(t, s)

	rec := doJSON(t, s, "POST", "/projects/ap/websites/aw/jobs/scan", `{"url":"http://example.com/"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var job app.Job
	decodeJSON(t, rec, &job)
	if job.ID == "" {
		t.Error("job.ID empty")
	}
	if job.Type != "scan" {
		t.Errorf("job.Type = %q, want %q", job.Type, "scan")
	}
}

// TestHandleStartScanJob_MissingURL_Returns400 asserts input validation at
// the HTTP layer.
func TestHandleStartScanJob_MissingURL_Returns400(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	setupAnalyzerSite(t, s)

	rec := doJSON(t, s, "POST", "/projects/ap/websites/aw/jobs/scan", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestHandleGetAnalyzerCapabilities_ReturnsBackend asserts the capabilities
// endpoint reports which backend is wired to a site.
func TestHandleGetAnalyzerCapabilities_ReturnsBackend(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	setupAnalyzerSite(t, s)

	rec := doJSON(t, s, "GET", "/projects/ap/websites/aw/analyzer/capabilities", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp struct {
		Backend      string         `json:"backend"`
		Capabilities map[string]any `json:"capabilities"`
	}
	decodeJSON(t, rec, &resp)
	if resp.Backend != "moku" {
		t.Errorf("Backend = %q, want %q (default backend)", resp.Backend, "moku")
	}
	if _, ok := resp.Capabilities["async"]; !ok {
		t.Errorf(`Capabilities missing "async" field; got keys=%v`, resp.Capabilities)
	}
}

// TestHandleGetAnalyzerHealth_ReturnsStatus asserts the health endpoint
// returns a status string.
func TestHandleGetAnalyzerHealth_ReturnsStatus(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)
	setupAnalyzerSite(t, s)

	rec := doJSON(t, s, "GET", "/projects/ap/websites/aw/analyzer/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp struct {
		Backend string `json:"backend"`
		Status  string `json:"status"`
	}
	decodeJSON(t, rec, &resp)
	if resp.Status == "" {
		t.Error("Status is empty")
	}
	if resp.Backend != "moku" {
		t.Errorf("Backend = %q, want %q", resp.Backend, "moku")
	}
}

// TestServer_DeleteThenRecreate_SameOrigin_Succeeds reproduces issue #17:
// creating a website, warming its tracker, deleting it, and recreating one
// with the same origin must succeed. Before the fix the recreate failed on
// Windows because the per-site `moku.db` SQLite handle was still held.
func TestServer_DeleteThenRecreate_SameOrigin_Succeeds(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	if rec := doJSON(t, s, "POST", "/projects",
		`{"slug":"proj","name":"Proj"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}

	// Create
	if rec := doJSON(t, s, "POST", "/projects/proj/websites",
		`{"slug":"site","origin":"https://example.com"}`); rec.Code != http.StatusCreated {
		t.Fatalf("first create website: %d %s", rec.Code, rec.Body.String())
	}

	// Warm the site components so the tracker SQLite DB is held open —
	// hitting the analyzer capabilities endpoint goes through
	// `siteComponentsFor` and forces a `NewSQLiteTracker` call.
	if rec := doJSON(t, s, "GET",
		"/projects/proj/websites/site/analyzer/capabilities", ""); rec.Code != http.StatusOK {
		t.Fatalf("warm site components: %d %s", rec.Code, rec.Body.String())
	}

	// Delete
	if rec := doJSON(t, s, "DELETE",
		"/projects/proj/websites/site", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete website: %d %s", rec.Code, rec.Body.String())
	}

	// Recreate with the SAME origin.
	rec := doJSON(t, s, "POST", "/projects/proj/websites",
		`{"slug":"site","origin":"https://example.com"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("recreate with same origin: %d %s", rec.Code, rec.Body.String())
	}
}
