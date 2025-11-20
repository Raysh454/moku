package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// Color coding to make test passes more satisfying
const green = "\033[32m"
const reset = "\033[0m"

// Depth 0
func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got / Request\n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, `
	<a href=/example>example</a>
	<a href=/blog>blog</a>
	`)
} 

// Depth 1
func getExample(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example Request\n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, `
	<a href=/example/a>example a</a>
	<a href=/example/b>example b</a>
	<a href=/example>example</a>
	`)
}

// Depth 2
func getExampleA(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/a Request\n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, `
	<a href=/example/a/1>example a 1</a>
	<a href=/blog>blog</a>
	`)
}

// Depth 2
func getExampleB(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/b Request\n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, `
	<a href=../example>test</a>
	`)
}

// Depth 3
func getExampleA1(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /example/a/1 Request\n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, "example/a/1")
}

// Depth 1
func getBlog(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Got /blog Request \n")
	w.Header().Add("Content-Type",  "text/html")
	io.WriteString(w, "blog")
}

func HttpServer(addr string) (*http.Server, error) {
	mux := http.NewServeMux()
	server := http.Server{
		Addr: addr,
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

func TestSpider(t *testing.T) {
	addr := "127.0.0.1:3333"
	server, err := HttpServer(addr)
	if err != nil {
		t.Errorf("Error setting up HttpServer: %v", err)
	}
	addr = "http://" + addr

	// Wait for server to start
	time.Sleep(2 * time.Second)

	urls := []string{
		addr,
		addr + "/example",
		addr + "/blog",
		addr + "/example/a",
		addr + "/example/b",
		addr + "/example/a/1",
	}

	// Create a simple webclient for testing
	cfg := &app.Config{WebClientBackend: "nethttp"}
	logger := logging.NewStdoutLogger("test")
	wc, err := webclient.NewNetHTTPClient(cfg, logger)
	if err != nil {
		return
	}
	
	fetcher, err := New("", 1, wc, nil)
	if err != nil {
		return
	}

	fmt.Print(fetcher.RootPath)

	fetcher.Fetch(urls)

	server.Shutdown(context.Background())
} 

