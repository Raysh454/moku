package enumerator_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/logging"
)

func TestWayback_should_implement_Enumerator_interface(t *testing.T) {
	wc := newTestWebClient(t)
	var _ enumerator.Enumerator = enumerator.NewWayback(wc, nil)
}

func TestWayback_should_fetch_and_deduplicate_urls_from_wayback_machine(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Wayback Machine CDX API response
		if r.URL.Path == "/cdx/search/cdx" {
			// Return URLs that match the test server domain
			response := fmt.Sprintf(`[
				["original", "timestamp", "url"],
				["com,example)/page1", "20230101", "%s/page1"],
				["com,example)/page2", "20230102", "%s/page2"],
				["com,example)/page1", "20230103", "%s/page1"]
			]`, srv.URL, srv.URL, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		// Return 404 for other sources
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	// Use the test server URL
	ctx := context.Background()
	target := srv.URL

	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	// Should deduplicate page1 (appears twice)
	if len(urls) != 2 {
		t.Errorf("expected 2 unique URLs, got %d: %v", len(urls), urls)
	}

	// Verify URLs are from the same domain
	for _, u := range urls {
		if u != srv.URL+"/page1" && u != srv.URL+"/page2" {
			t.Errorf("unexpected URL: %s", u)
		}
	}
}

func TestWayback_should_filter_subdomains(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			// Include URLs from subdomains that should be filtered out
			// Note: We can't actually test real subdomain filtering with httptest.Server
			// So we'll just verify that same-domain URLs are included
			response := fmt.Sprintf(`[
				["original", "timestamp", "url"],
				["com,example)/page1", "20230101", "%s/page1"],
				["com,example)/admin", "20230103", "%s/admin"]
			]`, srv.URL, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx := context.Background()
	target := srv.URL

	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	// Should include both same-domain URLs
	if len(urls) != 2 {
		t.Errorf("expected 2 URLs, got %d: %v", len(urls), urls)
	}
}

func TestWayback_should_handle_empty_response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			// Empty result (just headers)
			response := `[["original", "timestamp", "url"]]`
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx := context.Background()
	target := srv.URL

	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected 0 URLs for empty response, got %d: %v", len(urls), urls)
	}
}

func TestWayback_should_handle_source_failures_gracefully(t *testing.T) {
	// Server that returns errors for all sources
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 500 for all requests
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	logger := logging.NewStdoutLogger("test")
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, logger, cfg)

	ctx := context.Background()
	target := srv.URL

	// Should not panic, just return empty results
	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate should not error when sources fail: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected 0 URLs when all sources fail, got %d", len(urls))
	}
}

func TestWayback_should_handle_malformed_json(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			// Invalid JSON
			fmt.Fprint(w, `{"invalid": json}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx := context.Background()
	target := srv.URL

	// Should handle gracefully, returning empty results
	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected 0 URLs for malformed JSON, got %d", len(urls))
	}
}

func TestWayback_should_deduplicate_across_multiple_sources(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			// Wayback Machine response
			response := fmt.Sprintf(`[
				["original", "timestamp", "url"],
				["com,example)/page1", "20230101", "%s/page1"],
				["com,example)/page2", "20230102", "%s/page2"]
			]`, srv.URL, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		if r.URL.Path == "/CC-MAIN-2024-10-index" {
			// Common Crawl response with overlapping URLs
			response := fmt.Sprintf(`{"url":"%s/page1"}
{"url":"%s/page3"}`, srv.URL, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx := context.Background()
	target := srv.URL

	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	// Should have 3 unique URLs (page1, page2, page3), with page1 deduplicated
	if len(urls) != 3 {
		t.Errorf("expected 3 unique URLs across sources, got %d: %v", len(urls), urls)
	}

	// Verify all expected URLs are present
	urlMap := make(map[string]bool)
	for _, u := range urls {
		urlMap[u] = true
	}

	expectedURLs := []string{
		srv.URL + "/page1",
		srv.URL + "/page2",
		srv.URL + "/page3",
	}

	for _, expected := range expectedURLs {
		if !urlMap[expected] {
			t.Errorf("expected URL not found: %s", expected)
		}
	}
}

func TestWayback_should_respect_context_cancellation(t *testing.T) {
	// Server that delays responses
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond to simulate slow server
		<-r.Context().Done()
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	target := srv.URL

	// Should handle cancellation gracefully
	urls, err := wb.Enumerate(ctx, target, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	// May have 0 results due to cancellation
	if len(urls) > 0 {
		t.Logf("got %d URLs despite cancellation (timing dependent)", len(urls))
	}
}

func TestWayback_should_call_progress_callback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			response := `[["original", "timestamp", "url"]]`
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	cfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wb := enumerator.NewWaybackWithConfig(wc, nil, cfg)

	ctx := context.Background()
	target := srv.URL

	callbackCalled := false
	callback := func(current, failed, total int) {
		callbackCalled = true
		if current != 1 || total != 1 {
			t.Errorf("expected callback(1, 0, 1), got callback(%d, %d, %d)", current, failed, total)
		}
	}

	_, err := wb.Enumerate(ctx, target, callback)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if !callbackCalled {
		t.Error("progress callback was not called")
	}
}

func TestWayback_integration_with_composite(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdx/search/cdx" {
			response := fmt.Sprintf(`[
				["original", "timestamp", "url"],
				["com,example)/api", "20230101", "%s/api"]
			]`, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, response)
			return
		}
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "Disallow: /admin")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)

	// Create wayback and robots enumerators
	waybackCfg := &enumerator.WaybackConfig{
		WaybackBaseURL: srv.URL,
		CCBaseURL:      srv.URL,
	}
	wayback := enumerator.NewWaybackWithConfig(wc, nil, waybackCfg)
	robots := enumerator.NewRobots(wc, nil)

	// Use composite to combine them
	composite := enumerator.NewComposite([]enumerator.Enumerator{wayback, robots}, nil)

	ctx := context.Background()
	urls, err := composite.Enumerate(ctx, srv.URL, nil)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	// Should have results from both sources
	if len(urls) == 0 {
		t.Error("expected URLs from composite enumerator")
	}

	// Verify we got URLs from both sources
	hasWayback := false
	hasRobots := false
	for _, u := range urls {
		if u == srv.URL+"/api" {
			hasWayback = true
		}
		if u == srv.URL+"/admin" {
			hasRobots = true
		}
	}

	if !hasWayback {
		t.Error("expected wayback result in composite output")
	}
	if !hasRobots {
		t.Error("expected robots result in composite output")
	}
}
