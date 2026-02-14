package webclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/webclient"
)

// ─── Do: real HTTP round-trip via httptest ──────────────────────────────

func TestNetHTTPClient_Do_GET_ReturnsBody(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "response body")
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, err := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    ts.URL + "/test",
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "response body" {
		t.Errorf("expected 'response body', got %q", resp.Body)
	}
	if resp.Headers.Get("X-Custom") != "hello" {
		t.Errorf("expected X-Custom header 'hello', got %q", resp.Headers.Get("X-Custom"))
	}
}

func TestNetHTTPClient_Do_POST_SendsBody(t *testing.T) {
	t.Parallel()
	var receivedBody string
	var receivedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, err := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{
		Method: "POST",
		URL:    ts.URL + "/submit",
		Body:   []byte("payload"),
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedBody != "payload" {
		t.Errorf("expected body 'payload', got %q", receivedBody)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestNetHTTPClient_Do_ForwardsHeaders(t *testing.T) {
	t.Parallel()
	var receivedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, err := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	hdrs := http.Header{}
	hdrs.Set("Authorization", "Bearer test-token")

	_, err = client.Do(context.Background(), &webclient.Request{
		Method:  "GET",
		URL:     ts.URL,
		Headers: hdrs,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Authorization header forwarded, got %q", receivedAuth)
	}
}

func TestNetHTTPClient_Do_PropagatesStatusCode(t *testing.T) {
	t.Parallel()
	codes := []int{200, 301, 404, 500}

	for _, code := range codes {
		code := code
		t.Run(http.StatusText(code), func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer ts.Close()

			logger := &noopLogger{}
			httpClient := ts.Client()
			httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
			client, err := webclient.NewNetHTTPClient(webclient.Config{}, logger, httpClient)
			if err != nil {
				t.Fatalf("NewNetHTTPClient: %v", err)
			}
			defer client.Close()

			resp, err := client.Do(context.Background(), &webclient.Request{
				Method: "GET",
				URL:    ts.URL,
			})
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if resp.StatusCode != code {
				t.Errorf("expected %d, got %d", code, resp.StatusCode)
			}
		})
	}
}

func TestNetHTTPClient_Do_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	logger := &noopLogger{}
	client, _ := webclient.NewNetHTTPClient(webclient.Config{}, logger, nil)
	defer client.Close()

	_, err := client.Do(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestNetHTTPClient_Do_ConnectionRefused_ReturnsError(t *testing.T) {
	t.Parallel()
	logger := &noopLogger{}
	client, _ := webclient.NewNetHTTPClient(webclient.Config{}, logger, &http.Client{Timeout: 1 * time.Second})
	defer client.Close()

	_, err := client.Do(context.Background(), &webclient.Request{
		Method: "GET",
		URL:    "http://127.0.0.1:1", // port 1 is unlikely to be open
	})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestNetHTTPClient_Do_ContextCanceled_ReturnsError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, _ := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.Do(ctx, &webclient.Request{
		Method: "GET",
		URL:    ts.URL,
	})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

// ─── Get convenience method ────────────────────────────────────────────

func TestNetHTTPClient_Get_ReturnsBody(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		_, _ = io.WriteString(w, "get-response")
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, _ := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	defer client.Close()

	resp, err := client.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(resp.Body) != "get-response" {
		t.Errorf("expected 'get-response', got %q", resp.Body)
	}
}

// ─── Large response body ──────────────────────────────────────────────

func TestNetHTTPClient_Do_LargeBody(t *testing.T) {
	t.Parallel()
	largeBody := strings.Repeat("X", 1<<20) // 1 MiB
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, largeBody)
	}))
	defer ts.Close()

	logger := &noopLogger{}
	client, _ := webclient.NewNetHTTPClient(webclient.Config{}, logger, ts.Client())
	defer client.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{Method: "GET", URL: ts.URL})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(resp.Body) != 1<<20 {
		t.Errorf("expected 1MiB body, got %d bytes", len(resp.Body))
	}
}
