package harness

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// SSEStream is an open text/event-stream connection. Events are read on the
// caller's goroutine; the connection dies with its deadline or the test,
// whichever comes first.
type SSEStream struct {
	resp   *http.Response
	lines  *bufio.Scanner
	cancel context.CancelFunc
}

// OpenSSE connects to an SSE endpoint and returns once the server has
// accepted the stream (response headers received). The stream is force-closed
// when the deadline elapses, so a reader can never hang past it.
func OpenSSE(t *testing.T, rawURL string, deadline time.Duration) *SSEStream {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		t.Fatalf("harness: build SSE request %s: %v", rawURL, err)
	}

	streamingClient := &http.Client{} // no Timeout: it would kill the long-lived stream
	resp, err := streamingClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("harness: open SSE stream %s: %v", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		t.Fatalf("harness: SSE stream %s returned status %d", rawURL, resp.StatusCode)
	}

	stream := &SSEStream{
		resp:   resp,
		lines:  bufio.NewScanner(resp.Body),
		cancel: cancel,
	}
	t.Cleanup(stream.Close)
	return stream
}

// ReadEvent blocks until the next event arrives and returns its data payload.
// Comment and retry lines are skipped; multi-line data is joined with \n per
// the SSE specification.
func (s *SSEStream) ReadEvent() ([]byte, error) {
	var data []string
	for s.lines.Scan() {
		line := s.lines.Text()
		if line == "" {
			if len(data) > 0 {
				return []byte(strings.Join(data, "\n")), nil
			}
			continue // separator after a comment/retry line we ignored
		}
		if payload, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(payload, " "))
		}
	}
	if err := s.lines.Err(); err != nil {
		return nil, fmt.Errorf("SSE stream ended: %w", err)
	}
	return nil, fmt.Errorf("SSE stream closed by server")
}

// Close terminates the stream. Safe to call more than once.
func (s *SSEStream) Close() {
	s.cancel()
	s.resp.Body.Close()
}
