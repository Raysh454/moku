package enumerator_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raysh454/moku/internal/enumerator"
)

func TestRobots_should_implement_Enumerator_interface(t *testing.T) {
	wc := newTestWebClient(t)
	var _ enumerator.Enumerator = enumerator.NewRobots(wc, nil)
}

func TestRobots_should_extract_disallow_paths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /admin\nDisallow: /secret/page\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/admin", srv.URL + "/secret/page"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_extract_allow_paths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nAllow: /public\nDisallow: /private\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/public", srv.URL + "/private"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_extract_sitemap_directives(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/robots.txt" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "User-agent: *\nDisallow: /admin\nSitemap: %s/sitemap.xml\n", base)
		})
	})

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/admin", srv.URL + "/sitemap.xml"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_filter_cross_domain_sitemap_urls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /local\nSitemap: https://other.example.com/sitemap.xml\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/local"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_deduplicate_paths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /admin\nAllow: /admin\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/admin"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_return_empty_when_robots_not_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected empty slice, got %v", urls)
	}
}

func TestRobots_should_return_empty_for_empty_body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected empty slice, got %v", urls)
	}
}

func TestRobots_should_skip_comments_and_blank_lines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "# This is a comment\n\nUser-agent: *\n\n# another comment\nDisallow: /secret\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/secret"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_strip_trailing_wildcards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /api/*\nDisallow: /internal*\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)
	urls, err := robots.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/api", srv.URL + "/internal"}
	assertURLsEqual(t, urls, want)
}

func TestRobots_should_invoke_progress_callback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /admin\n")
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	robots := enumerator.NewRobots(wc, nil)

	var callCount int
	cb := func(processed, failed, total int) {
		callCount++
	}

	_, err := robots.Enumerate(context.Background(), srv.URL, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount == 0 {
		t.Error("expected progress callback to be called at least once")
	}
}
