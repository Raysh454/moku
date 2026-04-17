package server

import (
	"reflect"
	"testing"
)

// ─── parseAllowedOrigins ───────────────────────────────────────────────

func TestParseAllowedOrigins_SplitsCommaList(t *testing.T) {
	t.Parallel()
	got := parseAllowedOrigins("http://a.com,http://b.com")
	want := []string{"http://a.com", "http://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseAllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	got := parseAllowedOrigins("  http://a.com  ,   http://b.com   ")
	want := []string{"http://a.com", "http://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseAllowedOrigins_DropsEmptyEntries(t *testing.T) {
	t.Parallel()
	got := parseAllowedOrigins("http://a.com,,http://b.com,")
	want := []string{"http://a.com", "http://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseAllowedOrigins_TrimsOneTrailingSlash(t *testing.T) {
	t.Parallel()
	got := parseAllowedOrigins("http://a.com/,http://b.com")
	want := []string{"http://a.com", "http://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseAllowedOrigins_ReturnsEmptyForBlankInput(t *testing.T) {
	t.Parallel()
	got := parseAllowedOrigins("   ")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// ─── isOriginAllowed ───────────────────────────────────────────────────

func TestIsOriginAllowed_EmptyAllowlistAllowsAny(t *testing.T) {
	t.Parallel()
	if !isOriginAllowed("http://evil.example", nil) {
		t.Error("expected empty allowlist to be permissive (dev default)")
	}
}

func TestIsOriginAllowed_WildcardAllowsAny(t *testing.T) {
	t.Parallel()
	if !isOriginAllowed("http://evil.example", []string{"*"}) {
		t.Error("expected wildcard to allow any origin")
	}
}

func TestIsOriginAllowed_MissingOriginIsAllowed(t *testing.T) {
	t.Parallel()
	// Non-browser clients (curl, server-side Go) often omit Origin.
	if !isOriginAllowed("", []string{"http://trusted.example"}) {
		t.Error("expected empty Origin to be allowed (non-browser client)")
	}
}

func TestIsOriginAllowed_MatchesExactOrigin(t *testing.T) {
	t.Parallel()
	if !isOriginAllowed("http://trusted.example", []string{"http://trusted.example"}) {
		t.Error("expected exact match to be allowed")
	}
}

func TestIsOriginAllowed_RejectsOriginNotInAllowlist(t *testing.T) {
	t.Parallel()
	if isOriginAllowed("http://evil.example", []string{"http://trusted.example"}) {
		t.Error("expected unknown origin to be rejected")
	}
}

func TestIsOriginAllowed_CaseInsensitiveSchemeAndHost(t *testing.T) {
	t.Parallel()
	if !isOriginAllowed("HTTP://TRUSTED.EXAMPLE", []string{"http://trusted.example"}) {
		t.Error("expected case-insensitive match on scheme+host")
	}
}

func TestIsOriginAllowed_PortMatchMatters(t *testing.T) {
	t.Parallel()
	if isOriginAllowed("http://trusted.example:8080", []string{"http://trusted.example:9090"}) {
		t.Error("expected port mismatch to reject")
	}
	if !isOriginAllowed("http://trusted.example:8080", []string{"http://trusted.example:8080"}) {
		t.Error("expected exact port match to allow")
	}
}

func TestIsOriginAllowed_MalformedOriginIsRejected(t *testing.T) {
	t.Parallel()
	if isOriginAllowed("::::not-a-url", []string{"http://trusted.example"}) {
		t.Error("expected malformed Origin to be rejected")
	}
}

// ─── resolveAllowedOrigins ─────────────────────────────────────────────

func TestResolveAllowedOrigins_PrefersConfigOverEnv(t *testing.T) {
	t.Setenv("MOKU_ALLOWED_ORIGINS", "http://env.example")
	cfg := Config{AllowedOrigins: []string{"http://cfg.example"}}
	got := resolveAllowedOrigins(cfg)
	want := []string{"http://cfg.example"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveAllowedOrigins_FallsBackToEnvWhenConfigNil(t *testing.T) {
	t.Setenv("MOKU_ALLOWED_ORIGINS", "http://env.example,http://env2.example")
	cfg := Config{}
	got := resolveAllowedOrigins(cfg)
	want := []string{"http://env.example", "http://env2.example"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveAllowedOrigins_ReturnsEmptyWhenBothUnset(t *testing.T) {
	t.Setenv("MOKU_ALLOWED_ORIGINS", "")
	cfg := Config{}
	got := resolveAllowedOrigins(cfg)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
