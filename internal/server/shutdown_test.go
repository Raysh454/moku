package server_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestBeginShutdown_UnblocksSSEStreams proves that BeginShutdown closes the
// event broker so open SSE streams return promptly. Without it, the SSE write
// loop only exits on client disconnect, and a graceful HTTP shutdown (with no
// WriteTimeout) would block on every open stream until its context deadline.
func TestBeginShutdown_UnblocksSSEStreams(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/jobs/events", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	// A client with no timeout: the only thing that may end this read is the
	// server closing the stream.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	// The handler writes an initial "retry:" + ": connected" comment and
	// flushes immediately, so the first read returns as soon as the stream is
	// live. Block here until those first bytes land before triggering shutdown.
	buf := make([]byte, 1)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatalf("reading initial SSE bytes: %v", err)
	}

	// Drain the rest of the stream in a goroutine; signal when it hits EOF.
	eof := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		close(eof)
	}()

	srv.BeginShutdown()

	select {
	case <-eof:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE stream did not reach EOF within 2s after BeginShutdown")
	}
}
