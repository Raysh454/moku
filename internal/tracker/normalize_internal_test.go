package tracker

import (
	"reflect"
	"sort"
	"testing"
)

// ─── normalizeHeaders ──────────────────────────────────────────────────

func TestNormalizeHeaders_LowercasesNames(t *testing.T) {
	t.Parallel()
	input := map[string][]string{
		"Content-Type":  {"text/html"},
		"Cache-Control": {"no-cache"},
	}

	got := normalizeHeaders(input, false)

	if _, ok := got["content-type"]; !ok {
		t.Error("expected lowercased 'content-type'")
	}
	if _, ok := got["cache-control"]; !ok {
		t.Error("expected lowercased 'cache-control'")
	}
}

func TestNormalizeHeaders_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	input := map[string][]string{
		"Content-Type": {"  text/html  "},
	}

	got := normalizeHeaders(input, false)

	if got["content-type"][0] != "text/html" {
		t.Errorf("expected trimmed value, got %q", got["content-type"][0])
	}
}

func TestNormalizeHeaders_SortsMultiValueHeaders(t *testing.T) {
	t.Parallel()
	input := map[string][]string{
		"Accept": {"text/html", "application/json", "application/xml"},
	}

	got := normalizeHeaders(input, false)

	expected := []string{"application/json", "application/xml", "text/html"}
	if !reflect.DeepEqual(got["accept"], expected) {
		t.Errorf("expected sorted %v, got %v", expected, got["accept"])
	}
}

func TestNormalizeHeaders_RedactsSensitive(t *testing.T) {
	t.Parallel()
	input := map[string][]string{
		"Authorization": {"Bearer token123"},
		"Cookie":        {"session=xyz"},
		"X-Api-Key":     {"secret"},
		"Content-Type":  {"text/html"},
	}

	got := normalizeHeaders(input, true)

	for _, name := range []string{"authorization", "cookie", "x-api-key"} {
		if vals, ok := got[name]; !ok || len(vals) != 1 || vals[0] != "[REDACTED]" {
			t.Errorf("expected %s to be [REDACTED], got %v", name, vals)
		}
	}
	if got["content-type"][0] != "text/html" {
		t.Error("content-type should not be redacted")
	}
}

func TestNormalizeHeaders_NilInput(t *testing.T) {
	t.Parallel()
	got := normalizeHeaders(nil, false)

	if got == nil || len(got) != 0 {
		t.Errorf("expected empty map for nil input, got %v", got)
	}
}

func TestNormalizeHeaders_EmptyInput(t *testing.T) {
	t.Parallel()
	got := normalizeHeaders(map[string][]string{}, false)

	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestNormalizeHeaders_RemovesEmptyValues(t *testing.T) {
	t.Parallel()
	input := map[string][]string{
		"Content-Type": {"text/html", "", "  ", "application/json"},
	}

	got := normalizeHeaders(input, false)

	vals := got["content-type"]
	sort.Strings(vals)
	expected := []string{"application/json", "text/html"}
	if !reflect.DeepEqual(vals, expected) {
		t.Errorf("expected %v, got %v", expected, vals)
	}
}

// ─── isSensitiveHeader ─────────────────────────────────────────────────

func TestIsSensitiveHeader_KnownSensitive(t *testing.T) {
	t.Parallel()
	sensitive := []string{
		"authorization", "cookie", "set-cookie",
		"proxy-authorization", "www-authenticate",
		"proxy-authenticate", "x-api-key", "x-auth-token",
	}
	for _, h := range sensitive {
		if !isSensitiveHeader(h) {
			t.Errorf("expected %q to be sensitive", h)
		}
	}
}

func TestIsSensitiveHeader_NonSensitive(t *testing.T) {
	t.Parallel()
	nonSensitive := []string{
		"content-type", "cache-control", "accept", "user-agent",
	}
	for _, h := range nonSensitive {
		if isSensitiveHeader(h) {
			t.Errorf("expected %q to NOT be sensitive", h)
		}
	}
}

// ─── isOrderSensitiveHeader ────────────────────────────────────────────

func TestIsOrderSensitiveHeader(t *testing.T) {
	t.Parallel()
	orderSensitive := []string{"set-cookie", "www-authenticate", "proxy-authenticate"}
	for _, h := range orderSensitive {
		if !isOrderSensitiveHeader(h) {
			t.Errorf("expected %q to be order-sensitive", h)
		}
	}
	if isOrderSensitiveHeader("content-type") {
		t.Error("content-type should not be order-sensitive")
	}
}
