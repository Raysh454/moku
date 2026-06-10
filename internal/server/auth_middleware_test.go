package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/server"
	"github.com/raysh454/moku/internal/testutil"
)

// newTestServerWithToken builds a Server backed by a temp storage root and the
// supplied API token. An empty token leaves authentication disabled (no-op
// middleware).
func newTestServerWithToken(t *testing.T, apiToken string) *server.Server {
	t.Helper()
	cfg := server.Config{
		AppConfig: &app.Config{StorageRoot: t.TempDir()},
		Logger:    &testutil.DummyLogger{},
		APIToken:  apiToken,
	}
	s, err := server.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestAuthMiddleware_RejectsMissingToken(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "secret-token")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_AcceptsValidHeaderToken(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "secret-token")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("X-Moku-Token", "secret-token")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "secret-token")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("X-Moku-Token", "wrong-token")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_NoOpWhenTokenUnset(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "")

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when auth disabled, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_AllowsOptionsPreflight(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "secret-token")

	req := httptest.NewRequest(http.MethodOptions, "/projects", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected OPTIONS preflight to bypass auth, got 401")
	}
}

func TestAuthMiddleware_SSEAcceptsQueryParamToken(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	s := newTestServerWithToken(t, "secret-token")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/jobs/events?token=secret-token", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		s.ServeHTTP(rec, req)
		close(done)
	}()

	// The SSE handler streams until the request context is canceled; wait for it
	// to terminate so the recorder is fully written before asserting.
	<-done

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected SSE query-param token to authenticate, got 401 (body=%s)", rec.Body.String())
	}
}
