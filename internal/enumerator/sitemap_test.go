package enumerator_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

func newTestWebClient(t *testing.T) webclient.WebClient {
	t.Helper()
	cfg := webclient.Config{Client: webclient.ClientNetHTTP}
	logger := logging.NewStdoutLogger("test")
	wc, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("failed to create webclient: %v", err)
	}
	return wc
}

// sitemapServer creates an httptest.Server whose handler can reference the
// server's own URL. The caller provides a function that receives the server URL
// and returns the mux to use.
func sitemapServer(t *testing.T, setup func(baseURL string) http.Handler) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setup(srv.URL).ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSitemap_should_implement_Enumerator_interface(t *testing.T) {
	wc := newTestWebClient(t)
	var _ enumerator.Enumerator = enumerator.NewSitemap(wc, nil)
}

func TestSitemap_should_extract_urls_from_urlset(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/sitemap.xml" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/page1</loc></url>
  <url><loc>%s/page2</loc></url>
  <url><loc>%s/page3</loc></url>
</urlset>`, base, base, base)
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/page1", srv.URL + "/page2", srv.URL + "/page3"}
	assertURLsEqual(t, urls, want)
}

func TestSitemap_should_follow_sitemap_index_references(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			switch r.URL.Path {
			case "/sitemap.xml":
				fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s/sitemap1.xml</loc></sitemap>
  <sitemap><loc>%s/sitemap2.xml</loc></sitemap>
</sitemapindex>`, base, base)
			case "/sitemap1.xml":
				fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/alpha</loc></url>
</urlset>`, base)
			case "/sitemap2.xml":
				fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/beta</loc></url>
</urlset>`, base)
			default:
				http.NotFound(w, r)
			}
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/alpha", srv.URL + "/beta"}
	assertURLsEqual(t, urls, want)
}

func TestSitemap_should_try_common_sitemap_paths(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sitemap_index.xml" {
				w.Header().Set("Content-Type", "application/xml")
				fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/found-via-index</loc></url>
</urlset>`, base)
				return
			}
			http.NotFound(w, r)
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/found-via-index"}
	assertURLsEqual(t, urls, want)
}

func TestSitemap_should_filter_cross_domain_urls(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/sitemap.xml" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/local-page</loc></url>
  <url><loc>https://evil.example.com/phishing</loc></url>
</urlset>`, base)
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/local-page"}
	assertURLsEqual(t, urls, want)
}

func TestSitemap_should_return_empty_when_sitemap_not_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected empty slice, got %v", urls)
	}
}

func TestSitemap_should_handle_malformed_xml(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `this is not valid xml at all <><><>`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected empty slice, got %v", urls)
	}
}

func TestSitemap_should_invoke_progress_callback(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/sitemap.xml" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/a</loc></url>
</urlset>`, base)
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)

	var callCount int
	cb := func(processed, failed, total int) {
		callCount++
	}

	_, err := sitemap.Enumerate(context.Background(), srv.URL, cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount == 0 {
		t.Error("expected progress callback to be called at least once")
	}
}

func TestSitemap_should_deduplicate_urls(t *testing.T) {
	srv := sitemapServer(t, func(base string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/sitemap.xml" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/dup</loc></url>
  <url><loc>%s/dup</loc></url>
  <url><loc>%s/unique</loc></url>
</urlset>`, base, base, base)
		})
	})

	wc := newTestWebClient(t)
	sitemap := enumerator.NewSitemap(wc, nil)
	urls, err := sitemap.Enumerate(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{srv.URL + "/dup", srv.URL + "/unique"}
	assertURLsEqual(t, urls, want)
}

// assertURLsEqual compares two URL slices as sets (order-independent).
func assertURLsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, u := range want {
		wantSet[u] = struct{}{}
	}
	for _, u := range got {
		if _, ok := wantSet[u]; !ok {
			t.Errorf("unexpected URL in result: %s\ngot:  %v\nwant: %v", u, got, want)
		}
	}
}
