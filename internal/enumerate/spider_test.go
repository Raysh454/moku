package enumerate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
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

func AssertEqual(t *testing.T, maxDepth int, addr string, want []string, testNum, totalTests int) {
	t.Helper()

	spider := NewSpider(maxDepth)
	got, err := spider.Enumerate(addr)
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

	server.Shutdown(context.Background())
} 
