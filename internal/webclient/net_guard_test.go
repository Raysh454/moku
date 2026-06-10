package webclient_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

// TestGet_BlocksLoopbackByDefault verifies that a default-configured client
// (no AllowPrivateHosts) refuses to dial a loopback httptest server because the
// dialer guard rejects the resolved private IP.
func TestGet_BlocksLoopbackByDefault(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client, err := webclient.NewNetHTTPClient(webclient.Config{}, &noopLogger{}, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), ts.URL)
	if !errors.Is(err, webclient.ErrPrivateHostBlocked) {
		t.Fatalf("expected ErrPrivateHostBlocked, got %v", err)
	}
}

// TestGet_AllowsLoopbackWhenAllowPrivateHosts verifies that the escape hatch
// (AllowPrivateHosts) lets the client reach a loopback server.
func TestGet_AllowsLoopbackWhenAllowPrivateHosts(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer ts.Close()

	client, err := webclient.NewNetHTTPClient(
		webclient.Config{AllowPrivateHosts: true}, &noopLogger{}, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	resp, err := client.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestGet_RejectsOversizedBody verifies that a response exceeding MaxBodyBytes
// fails loud with ErrBodyTooLarge rather than silently truncating.
func TestGet_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	const bodySize = 4 << 10 // 4 KiB
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("X", bodySize))
	}))
	defer ts.Close()

	client, err := webclient.NewNetHTTPClient(
		webclient.Config{AllowPrivateHosts: true, MaxBodyBytes: 1024}, &noopLogger{}, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), ts.URL)
	if !errors.Is(err, webclient.ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

// TestGet_StopsRedirectLoops verifies that an infinite self-redirect terminates
// with an error instead of looping forever.
func TestGet_StopsRedirectLoops(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer ts.Close()

	client, err := webclient.NewNetHTTPClient(
		webclient.Config{AllowPrivateHosts: true}, &noopLogger{}, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient: %v", err)
	}
	defer client.Close()

	_, err = client.Get(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected an error for an infinite redirect loop, got nil")
	}
}
