package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/raysh454/moku/internal/enumerate"
)

func setupHttpServer() *httptest.Server {
	mux := http.NewServeMux()

	// Root page links to /page1 and /page2
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="http://%s/page1">Page 1</a> <a href="http://%s/page2">Page 2</a>`, r.Host, r.Host)
	})

	// /page1 links to /page3
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="http://%s/page3">Page 3</a>`, r.Host)
	})

	// /page2 has no links
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "This is page 2")
	})

	// /page3 has no links
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "This is page 3")
	})

	return httptest.NewServer(mux)
}

func main() {
	server := setupHttpServer()
	defer server.Close() // Close AFTER crawling

	spider := enumerate.NewSpider(2)
	result, err := spider.Enumerate(server.URL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Printf("got: %v\n", result)
}

