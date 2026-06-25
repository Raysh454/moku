package webclient_test

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

func newTestTLSClient(t *testing.T, cfg webclient.Config) webclient.WebClient {
	t.Helper()
	client, err := webclient.NewTLSClient(cfg, &noopLogger{})
	if err != nil {
		t.Fatalf("NewTLSClient: %v", err)
	}
	return client
}

func TestNewTLSClient_Construct(t *testing.T) {
	t.Parallel()
	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS})
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	defer client.Close()
}

func TestTLSClient_Do_GET_ReturnsStatusHeadersBody(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "real content")
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	resp, err := client.Do(context.Background(), &webclient.Request{Method: "GET", URL: ts.URL + "/x"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "real content" {
		t.Errorf("expected body 'real content', got %q", resp.Body)
	}
	if resp.Headers.Get("X-Custom") != "hello" {
		t.Errorf("expected X-Custom 'hello', got %q", resp.Headers.Get("X-Custom"))
	}
}

func TestTLSClient_Do_SetsChromeUserAgentWhenAbsent(t *testing.T) {
	t.Parallel()
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	if _, err := client.Get(context.Background(), ts.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(gotUA, "Chrome/146") {
		t.Errorf("expected a Chrome/146 User-Agent consistent with the TLS profile, got %q", gotUA)
	}
}

func TestTLSClient_Do_ForwardsCallerHeadersAndUserAgentOverride(t *testing.T) {
	t.Parallel()
	var gotAuth, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.UserAgent()
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer test-token")
	hdr.Set("User-Agent", "custom-agent/1.0")

	if _, err := client.Do(context.Background(), &webclient.Request{Method: "GET", URL: ts.URL, Headers: hdr}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization forwarded, got %q", gotAuth)
	}
	if gotUA != "custom-agent/1.0" {
		t.Errorf("expected caller User-Agent to override the default, got %q", gotUA)
	}
}

func TestTLSClient_Do_TransparentlyDecompressesGzip(t *testing.T) {
	t.Parallel()
	const want = "<html>decompressed body</html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "text/html")
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, want)
		_ = gz.Close()
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	resp, err := client.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(resp.Body) != want {
		t.Errorf("expected gzip body to be decompressed to %q, got %q", want, resp.Body)
	}
}

func TestTLSClient_Do_BodyCap_RejectsOversize(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("X", 100))
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true, MaxBodyBytes: 8})
	defer client.Close()

	_, err := client.Get(context.Background(), ts.URL)
	if !errors.Is(err, webclient.ErrBodyTooLarge) {
		t.Errorf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestTLSClient_Do_SSRFGuard_BlocksLoopback(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// AllowPrivateHosts defaults to false, so the shared dial guard must refuse
	// the loopback httptest address.
	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS})
	defer client.Close()

	_, err := client.Get(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected the SSRF guard to block a loopback host")
	}
	if !errors.Is(err, webclient.ErrPrivateHostBlocked) {
		t.Errorf("expected ErrPrivateHostBlocked, got %v", err)
	}
}

func TestTLSClient_Do_RedirectCap_StopsRunawayChain(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	if _, err := client.Get(context.Background(), ts.URL); err == nil {
		t.Fatal("expected an error once the redirect cap is exceeded")
	}
}

func TestTLSClient_Do_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	if _, err := client.Do(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestTLSClient_Do_ContextCanceled_ReturnsError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := newTestTLSClient(t, webclient.Config{Client: webclient.ClientTLS, AllowPrivateHosts: true})
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Get(ctx, ts.URL); err == nil {
		t.Fatal("expected error for canceled context")
	}
}
