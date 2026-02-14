package utils_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// ─── NewSnapshotFromResponse ───────────────────────────────────────────

func TestNewSnapshotFromResponse_NilReturnsNil(t *testing.T) {
	t.Parallel()
	if snap := utils.NewSnapshotFromResponse(nil); snap != nil {
		t.Errorf("expected nil, got %+v", snap)
	}
}

func TestNewSnapshotFromResponse_BasicFields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	resp := &webclient.Response{
		Request:    &webclient.Request{Method: "GET", URL: "https://example.com/page"},
		Body:       []byte("<html>body</html>"),
		Headers:    http.Header{"Content-Type": {"text/html"}},
		StatusCode: 200,
		FetchedAt:  now,
	}

	snap := utils.NewSnapshotFromResponse(resp)

	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.URL != "https://example.com/page" {
		t.Errorf("expected URL from request, got %q", snap.URL)
	}
	if snap.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", snap.StatusCode)
	}
	if string(snap.Body) != "<html>body</html>" {
		t.Errorf("unexpected body: %q", snap.Body)
	}
	if snap.CreatedAt != now {
		t.Errorf("expected CreatedAt = FetchedAt")
	}
}

func TestNewSnapshotFromResponse_LowercasesHeaderKeys(t *testing.T) {
	t.Parallel()
	resp := &webclient.Response{
		Request:    &webclient.Request{URL: "https://example.com"},
		Body:       []byte(""),
		Headers:    http.Header{"Content-Type": {"text/html"}, "X-Custom-Header": {"val"}},
		StatusCode: 200,
		FetchedAt:  time.Now(),
	}

	snap := utils.NewSnapshotFromResponse(resp)

	if _, ok := snap.Headers["content-type"]; !ok {
		t.Error("expected lowercased 'content-type' header key")
	}
	if _, ok := snap.Headers["x-custom-header"]; !ok {
		t.Error("expected lowercased 'x-custom-header' header key")
	}
}

func TestNewSnapshotFromResponse_IDLeftEmpty(t *testing.T) {
	t.Parallel()
	resp := &webclient.Response{
		Request:    &webclient.Request{URL: "https://example.com"},
		Body:       []byte(""),
		StatusCode: 200,
		FetchedAt:  time.Now(),
	}

	snap := utils.NewSnapshotFromResponse(resp)
	if snap.ID != "" {
		t.Errorf("expected empty ID (assigned by tracker), got %q", snap.ID)
	}
}

// ─── URLTools ──────────────────────────────────────────────────────────

func TestNewURLTools_ValidURL(t *testing.T) {
	t.Parallel()
	u, err := utils.NewURLTools("https://example.com/path")
	if err != nil {
		t.Fatalf("NewURLTools: %v", err)
	}
	if u.URL.Hostname() != "example.com" {
		t.Errorf("expected hostname 'example.com', got %q", u.URL.Hostname())
	}
}

func TestNewURLTools_NormalizesSchemeAndHost(t *testing.T) {
	t.Parallel()
	u, err := utils.NewURLTools("HTTPS://EXAMPLE.COM:443/Page")
	if err != nil {
		t.Fatalf("NewURLTools: %v", err)
	}
	if u.URL.Scheme != "https" {
		t.Errorf("expected lowercased scheme, got %q", u.URL.Scheme)
	}
	if u.URL.Host != "example.com" {
		t.Errorf("expected default port stripped, got host %q", u.URL.Host)
	}
}

func TestNewURLTools_StripsFragment(t *testing.T) {
	t.Parallel()
	u, err := utils.NewURLTools("https://example.com/page#section")
	if err != nil {
		t.Fatalf("NewURLTools: %v", err)
	}
	if u.URL.Fragment != "" {
		t.Errorf("expected empty fragment, got %q", u.URL.Fragment)
	}
}

func TestNewURLTools_StripsTrailingSlash(t *testing.T) {
	t.Parallel()
	u, err := utils.NewURLTools("https://example.com/path/")
	if err != nil {
		t.Fatalf("NewURLTools: %v", err)
	}
	if u.URL.Path != "/path" {
		t.Errorf("expected /path, got %q", u.URL.Path)
	}
}

func TestURLTools_DomainIsSame(t *testing.T) {
	t.Parallel()
	a, _ := utils.NewURLTools("https://example.com/a")
	b, _ := utils.NewURLTools("https://example.com/b")
	c, _ := utils.NewURLTools("https://other.com/a")

	if !a.DomainIsSame(b) {
		t.Error("expected same domain for example.com paths")
	}
	if a.DomainIsSame(c) {
		t.Error("expected different domain for example.com vs other.com")
	}
}

func TestURLTools_DomainIsSameString(t *testing.T) {
	t.Parallel()
	u, _ := utils.NewURLTools("https://example.com")

	same, err := u.DomainIsSameString("https://example.com/page")
	if err != nil {
		t.Fatalf("DomainIsSameString: %v", err)
	}
	if !same {
		t.Error("expected same domain")
	}

	diff, err := u.DomainIsSameString("https://other.com")
	if err != nil {
		t.Fatalf("DomainIsSameString: %v", err)
	}
	if diff {
		t.Error("expected different domain")
	}
}

func TestURLTools_GetPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url      string
		wantPath string
	}{
		{"https://example.com/api/v1", "/api/v1"},
		{"https://example.com", ""},
	}

	for _, tt := range tests {
		u, _ := utils.NewURLTools(tt.url)
		got := u.GetPath()
		if got != tt.wantPath {
			t.Errorf("GetPath(%q) = %q, want %q", tt.url, got, tt.wantPath)
		}
	}
}

func TestURLTools_ResolveFullUrlString(t *testing.T) {
	t.Parallel()
	base, _ := utils.NewURLTools("https://example.com/app")

	resolved, err := base.ResolveFullUrlString("/static")
	if err != nil {
		t.Fatalf("ResolveFullUrlString: %v", err)
	}
	// normalize() strips trailing slashes, so the result has no trailing slash
	if resolved != "https://example.com/static" {
		t.Errorf("expected 'https://example.com/static', got %q", resolved)
	}
}

// ─── Canonicalize additional cases ─────────────────────────────────────

func TestCanonicalize_DefaultScheme(t *testing.T) {
	t.Parallel()
	opts := utils.CanonicalizeOptions{DefaultScheme: "https"}
	got, err := utils.Canonicalize("example.com/page", opts)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com/page" {
		t.Errorf("expected https://example.com/page, got %q", got)
	}
}

func TestCanonicalize_EmptyURL_Error(t *testing.T) {
	t.Parallel()
	_, err := utils.Canonicalize("", utils.CanonicalizeOptions{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestCanonicalize_StripTrailingSlash(t *testing.T) {
	t.Parallel()
	opts := utils.CanonicalizeOptions{StripTrailingSlash: true}
	got, err := utils.Canonicalize("https://example.com/path/", opts)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com/path" {
		t.Errorf("expected trailing slash stripped, got %q", got)
	}
}

func TestCanonicalize_SortQueryParams(t *testing.T) {
	t.Parallel()
	got, err := utils.Canonicalize("https://example.com?z=1&a=2", utils.CanonicalizeOptions{})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	// path.Clean("") → "/", so canonical URL always has a root path
	if got != "https://example.com/?a=2&z=1" {
		t.Errorf("expected sorted params, got %q", got)
	}
}

func TestCanonicalize_DropTrackingParams(t *testing.T) {
	t.Parallel()
	opts := utils.CanonicalizeOptions{DropTrackingParams: true}
	got, err := utils.Canonicalize("https://example.com?page=1&utm_source=google", opts)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com/?page=1" {
		t.Errorf("expected tracking param dropped, got %q", got)
	}
}

func TestCanonicalize_LowercasesSchemeAndHost(t *testing.T) {
	t.Parallel()
	got, err := utils.Canonicalize("HTTPS://EXAMPLE.COM/Path", utils.CanonicalizeOptions{})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com/Path" {
		t.Errorf("expected lowercased scheme+host, got %q", got)
	}
}

func TestCanonicalize_DefaultPortStripped(t *testing.T) {
	t.Parallel()
	got, err := utils.Canonicalize("https://example.com:443/page", utils.CanonicalizeOptions{})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com/page" {
		t.Errorf("expected port 443 stripped, got %q", got)
	}
}

func TestCanonicalize_NonDefaultPortPreserved(t *testing.T) {
	t.Parallel()
	got, err := utils.Canonicalize("https://example.com:8443/page", utils.CanonicalizeOptions{})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if got != "https://example.com:8443/page" {
		t.Errorf("expected port preserved, got %q", got)
	}
}
