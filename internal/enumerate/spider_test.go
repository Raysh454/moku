package enumerate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Dummy implementation of utils.NewURLTools for testing
// Only needed if you're not mocking utils in tests.

func TestSpider_Enumerate(t *testing.T) {
	// Set up a simple test server
	mux := http.NewServeMux()

	// Root page links to /page1 and /page2
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s/page1">Page 1</a> <a href="%s/page2">Page 2</a>`, r.Host, r.Host)
	})

	// /page1 links to /page3
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s/page3">Page 3</a>`, r.Host)
	})

	// /page2 has no links
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "This is page 2")
	})

	// /page3 has no links
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "This is page 3")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Add scheme to URLs in HTML responses
	serverURL := "http://" + server.Listener.Addr().String()

	t.Run("depth 1", func(t *testing.T) {
		spider := NewSpider(1)
		got, err := spider.Enumerate(serverURL)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		expected := []string{
			serverURL,
			serverURL + "/page1",
			serverURL + "/page2",
		}

		assertURLs(t, got, expected)
	})

	t.Run("depth 2", func(t *testing.T) {
		spider := NewSpider(2)
		got, err := spider.Enumerate(serverURL)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		expected := []string{
			serverURL,
			serverURL + "/page1",
			serverURL + "/page2",
			serverURL + "/page3",
		}

		assertURLs(t, got, expected)
	})
}

// Helper to check expected vs actual slices
func assertURLs(t *testing.T, got, expected []string) {
	if len(got) != len(expected) {
		t.Errorf("Expected %d URLs, got %d\nExpected: %v\nGot: %v", len(expected), len(got), expected, got)
		return
	}

	seen := map[string]bool{}
	for _, url := range got {
		seen[url] = true
	}

	for _, url := range expected {
		if !seen[url] {
			t.Errorf("Expected URL missing: %s", url)
		}
	}
}

