package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func runCorsMiddleware(t *testing.T, allowed []string, origin string) http.Header {
	t.Helper()
	s := &Server{allowedOrigins: allowed}
	var inner http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	req := httptest.NewRequest(http.MethodGet, "/anywhere", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	s.corsMiddleware(inner).ServeHTTP(rec, req)
	return rec.Header()
}

func TestCorsMiddleware_SendsWildcard_WhenAllowlistEmpty(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, nil, "http://whatever.example")
	if got.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected *, got %q", got.Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsMiddleware_SendsWildcard_WhenAllowlistContainsStar(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, []string{"*"}, "http://whatever.example")
	if got.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected *, got %q", got.Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsMiddleware_ReflectsOrigin_WhenInAllowlist(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, []string{"http://trusted.example"}, "http://trusted.example")
	if got.Get("Access-Control-Allow-Origin") != "http://trusted.example" {
		t.Errorf("expected reflected origin, got %q", got.Get("Access-Control-Allow-Origin"))
	}
	if got.Get("Vary") != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", got.Get("Vary"))
	}
}

func TestCorsMiddleware_OmitsACAO_WhenOriginNotInAllowlist(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, []string{"http://trusted.example"}, "http://evil.example")
	if v := got.Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no ACAO header, got %q", v)
	}
}

func TestCorsMiddleware_OmitsACAO_WhenNoOriginHeaderAndAllowlistIsSet(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, []string{"http://trusted.example"}, "")
	if v := got.Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no ACAO header for same-origin/non-browser, got %q", v)
	}
}

func TestCorsMiddleware_AlwaysSetsHeadersAndMaxAge(t *testing.T) {
	t.Parallel()
	got := runCorsMiddleware(t, []string{"http://trusted.example"}, "http://evil.example")
	if got.Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers to be set regardless of allowlist result")
	}
	if got.Get("Access-Control-Max-Age") == "" {
		t.Error("expected Access-Control-Max-Age to be set regardless of allowlist result")
	}
}
