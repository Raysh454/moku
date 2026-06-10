package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	pollEvery      = 250 * time.Millisecond
)

// Client speaks JSON-over-HTTP to a process under test. It is the only
// transport the acceptance suite has — if the product can't be driven through
// it, the product is missing surface, not the tests.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: requestTimeout},
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

// Request performs one JSON round trip. body and out may be nil.
func (c *Client) Request(method, path string, body, out any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request body for %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body %s %s: %w", method, path, err)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, respBody, fmt.Errorf("decode response body %s %s: %w; body=%s", method, path, err, respBody)
		}
	}

	return resp.StatusCode, respBody, nil
}

// MustStatus performs a request and fails the test unless it returns the
// expected HTTP status.
func (c *Client) MustStatus(t *testing.T, expected int, method, path string, body, out any) []byte {
	t.Helper()
	code, respBody, err := c.Request(method, path, body, out)
	if err != nil {
		t.Fatalf("%s %s request failed: %v", method, path, err)
	}
	if code != expected {
		t.Fatalf("%s %s expected status %d got %d; body=%s", method, path, expected, code, respBody)
	}
	return respBody
}

// WaitUntil polls fn until it reports success or the timeout elapses, then
// fails the test with fn's last message and error.
func WaitUntil(t *testing.T, timeout time.Duration, fn func() (bool, string, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastMsg string
	var lastErr error

	for time.Now().Before(deadline) {
		ok, msg, err := fn()
		if ok {
			return
		}
		lastMsg = msg
		lastErr = err
		time.Sleep(pollEvery)
	}

	if lastErr != nil {
		t.Fatalf("timed out waiting: %s; last error: %v", lastMsg, lastErr)
	}
	t.Fatalf("timed out waiting: %s", lastMsg)
}
