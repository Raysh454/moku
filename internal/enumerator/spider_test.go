package enumerator_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// Color coding to make test passes more satisfying
const green = "\033[32m"
const reset = "\033[0m"

// Depth 0
func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got / Request\n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, `
	<a href=/example>example</a>
	<a href=/blog>blog</a>
	`); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

// Depth 1
func getExample(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example Request\n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, `
	<a href=/example/a>example a</a>
	<a href=/example/b>example b</a>
	<a href=/example>example</a>
	`); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

// Depth 2
func getExampleA(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/a Request\n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, `
	<a href=/example/a/1>example a 1</a>
	<a href=/blog>blog</a>
	`); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

// Depth 2
func getExampleB(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/b Request\n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, `
	<a href=../example>test</a>
	`); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

// Depth 3
func getExampleA1(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/a/1 Request\n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, "example/a/1"); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

// Depth 1
func getBlog(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /blog Request \n")
	w.Header().Add("Content-Type", "text/html")
	if _, err := io.WriteString(w, "blog"); err != nil {
		fmt.Printf("Error writing response: %v", err)
	}
}

func HttpServer(addr string) (*http.Server, error) {
	mux := http.NewServeMux()
	server := http.Server{
		Addr:    addr,
		Handler: mux,
	}

	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/example", getExample)
	mux.HandleFunc("/example/a", getExampleA)
	mux.HandleFunc("/example/b", getExampleB)
	mux.HandleFunc("/example/a/1", getExampleA1)
	mux.HandleFunc("/blog", getBlog)

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("err: %v", err)
		}
	}()

	return &server, nil
}

func AssertEqual(t *testing.T, maxDepth int, addr string, want []string, testNum, totalTests int) {
	t.Helper()

	// Create a simple webclient for testing. AllowPrivateHosts is required
	// because the test server is on loopback, which the SSRF guard blocks.
	cfg := webclient.Config{Client: webclient.ClientNetHTTP, AllowPrivateHosts: true}
	logger := logging.NewStdoutLogger("test")
	wc, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create webclient: %v", err)
	}

	spider := enumerator.NewSpider(maxDepth, wc, nil)
	got, err := spider.Enumerate(context.Background(), addr, nil)
	if err != nil {
		t.Errorf("error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want: %v", got, want)
	} else {
		fmt.Printf("Got: %v\n", got)
		fmt.Printf("%sPASS %d/%d%s\n", green, testNum, totalTests, reset)
	}
}

// TestSpider_StopsAtMaxPages verifies that the spider honors a configured
// MaxPages cap: even when a deep chain of same-domain pages is reachable within
// MaxDepth, the crawl stops once MaxPages results have been collected.
func TestSpider_StopsAtMaxPages(t *testing.T) {
	// Each /page{n} links to /page{n+1}, forming a long single-domain chain.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		next := r.URL.Path + "/next"
		fmt.Fprintf(w, `<a href=%q>next</a>`, next)
	}))
	defer ts.Close()

	cfg := webclient.Config{Client: webclient.ClientNetHTTP, AllowPrivateHosts: true}
	wc, err := webclient.NewNetHTTPClient(cfg, logging.NewStdoutLogger("test"), nil)
	if err != nil {
		t.Fatalf("failed to create webclient: %v", err)
	}

	const maxPages = 3
	spider := enumerator.NewSpider(100, wc, nil)
	spider.MaxPages = maxPages

	got, err := spider.Enumerate(context.Background(), ts.URL, nil)
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}

	if len(got) > maxPages {
		t.Errorf("spider returned %d pages, want at most %d: %v", len(got), maxPages, got)
	}
}

func TestSpider(t *testing.T) {
	addr := "127.0.0.1:3333"
	server, err := HttpServer(addr)
	if err != nil {
		t.Errorf("Error setting up HttpServer: %v", err)
	}
	addr = "http://" + addr

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Depth 0 test
	fmt.Printf("Testing Depth 0\n")
	want := []string{
		addr,
		addr + "/example",
		addr + "/blog",
	}
	AssertEqual(t, 0, addr, want, 1, 3)

	// Depth 1 test
	fmt.Printf("Testing Depth 1\n")
	want = []string{
		addr,
		addr + "/example",
		addr + "/blog",
		addr + "/example/a",
		addr + "/example/b",
	}
	AssertEqual(t, 1, addr, want, 2, 3)

	// Depth 2 test
	fmt.Printf("Testing Depth 2\n")
	want = []string{
		addr,
		addr + "/example",
		addr + "/blog",
		addr + "/example/a",
		addr + "/example/b",
		addr + "/example/a/1",
	}

	AssertEqual(t, 2, addr, want, 3, 3)

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("server shutdown: %v", err)
	}
}
