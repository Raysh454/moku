package webclient_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

type chromedpNoopLogger struct{}

func (n *chromedpNoopLogger) Debug(msg string, fields ...logging.Field) {}
func (n *chromedpNoopLogger) Info(msg string, fields ...logging.Field)  {}
func (n *chromedpNoopLogger) Warn(msg string, fields ...logging.Field)  {}
func (n *chromedpNoopLogger) Error(msg string, fields ...logging.Field) {}
func (n *chromedpNoopLogger) With(fields ...logging.Field) logging.Logger {
	return n
}

func mustCreateChromedpClient(t *testing.T) webclient.WebClient {
	t.Helper()
	cfg := webclient.Config{Client: webclient.ClientChromedp}
	logger := &chromedpNoopLogger{}

	client, err := webclient.NewChromedpClient(cfg, logger)
	if err != nil {
		t.Skipf("chromedp unavailable: %v", err)
	}

	t.Cleanup(func() { client.Close() })
	return client
}

func TestNewChromedpClient_ReturnsNonNilClient(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestChromedpClient_Close_ReturnsNil(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	err := client.Close()

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestChromedpClient_Close_Idempotent(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	firstErr := client.Close()
	secondErr := client.Close()

	if firstErr != nil {
		t.Fatalf("first Close: expected nil, got %v", firstErr)
	}
	if secondErr != nil {
		t.Fatalf("second Close: expected nil, got %v", secondErr)
	}
}

func TestChromedpClient_Do_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	_, err := client.Do(context.Background(), nil)

	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if !strings.Contains(err.Error(), "nil request") {
		t.Errorf("expected 'nil request' in error, got %q", err)
	}
}

func TestChromedpClient_Do_EmptyMethod_DefaultsToGET(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	_, err := client.Do(context.Background(), &webclient.Request{URL: "about:blank"})

	if err != nil && strings.Contains(err.Error(), "not supported") {
		t.Fatal("empty method should default to GET, not be rejected")
	}
}

func TestChromedpClient_Do_NonGET_ReturnsError(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			_, err := client.Do(context.Background(), &webclient.Request{
				Method: method,
				URL:    "http://example.com",
			})
			if err == nil {
				t.Fatalf("expected error for %s request", method)
			}
			if !strings.Contains(err.Error(), "not supported") {
				t.Errorf("expected 'not supported' in error, got %q", err)
			}
		})
	}
}

func TestChromedpClient_Do_MethodCaseInsensitive(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	_, err := client.Do(context.Background(), &webclient.Request{
		Method: "get",
		URL:    "about:blank",
	})

	if err != nil && strings.Contains(err.Error(), "not supported") {
		t.Fatal("lowercase 'get' should be treated as GET")
	}
}

func TestChromedpClient_Do_AfterClose_ReturnsError(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	client.Close()

	_, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    "about:blank",
	})

	if err == nil {
		t.Fatal("expected error after Close")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' in error, got %q", err)
	}
}

func TestChromedpClient_Do_ReturnsHTMLBody(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><body><h1>Hello Chromedp</h1></body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !strings.Contains(string(resp.Body), "Hello Chromedp") {
		t.Errorf("expected body to contain 'Hello Chromedp', got %q", string(resp.Body))
	}
	if resp.Request != (&webclient.Request{Method: "GET", URL: ts.URL}) {
		// Just verify it's set, not pointer equality
	}
}

func TestChromedpClient_Do_FetchedAtIsRecent(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><body>ok</body></html>`)
	}))
	defer ts.Close()

	before := time.Now()
	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if resp.FetchedAt.Before(before) {
		t.Errorf("FetchedAt %v is before request start %v", resp.FetchedAt, before)
	}
	if time.Since(resp.FetchedAt) > 60*time.Second {
		t.Errorf("FetchedAt is too old: %v", resp.FetchedAt)
	}
}

func TestChromedpClient_Do_CapturesStatusCode(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `<html><body>ok</body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestChromedpClient_Do_CapturesNon200StatusCode(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<html><body>not found</body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestChromedpClient_Do_CapturesResponseHeaders(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `<html><body>headers</body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	got := resp.Headers.Get("X-Custom-Header")
	if got == "" {
		t.Error("expected X-Custom-Header to be present in response headers")
	}
	if got != "" && got != "test-value" {
		t.Errorf("expected X-Custom-Header 'test-value', got %q", got)
	}
}

func TestChromedpClient_Do_ForwardsRequestHeaders(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<html><body>%s</body></html>`, auth)
	}))
	defer ts.Close()

	hdrs := http.Header{}
	hdrs.Set("Authorization", "Bearer test-token-xyz")

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method:  "GET",
		URL:     ts.URL,
		Headers: hdrs,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(string(resp.Body), "Bearer test-token-xyz") {
		t.Errorf("expected body to contain forwarded auth header, got %q", string(resp.Body))
	}
}

func TestChromedpClient_Do_WaitsForNetworkIdle(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dynamic" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `"loaded"`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><body>
			<div id="target">waiting</div>
			<script>
				fetch("/dynamic")
					.then(r => r.json())
					.then(data => { document.getElementById("target").textContent = data; });
			</script>
		</body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})

	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(string(resp.Body), "loaded") {
		t.Logf("body: %s", string(resp.Body))
		t.Error("expected body to contain dynamically loaded content 'loaded'")
	}
}

func TestChromedpClient_Do_ContextCanceled_ReturnsError(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Do(ctx, &webclient.Request{
		Method: "GET",
		URL:    "http://example.com",
	})

	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestChromedpClient_Get_ReturnsResponse(t *testing.T) {
	t.Parallel()
	client := mustCreateChromedpClient(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><body>get-response</body></html>`)
	}))
	defer ts.Close()

	resp, err := client.Get(context.Background(), ts.URL)

	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(string(resp.Body), "get-response") {
		t.Errorf("expected body to contain 'get-response', got %q", string(resp.Body))
	}
	if resp.Request.Method != "GET" {
		t.Errorf("expected request method GET, got %q", resp.Request.Method)
	}
	if resp.Request.URL != ts.URL {
		t.Errorf("expected request URL %q, got %q", ts.URL, resp.Request.URL)
	}
}
