package server

import (
	"net/url"
	"os"
	"slices"
	"strings"
)

const allowedOriginsEnvVar = "MOKU_ALLOWED_ORIGINS"

// parseAllowedOrigins converts a comma-separated origin string into a normalized
// slice: whitespace trimmed, empty entries dropped, one trailing slash removed
// to reduce config footguns.
func parseAllowedOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimSuffix(trimmed, "/")
		out = append(out, trimmed)
	}
	return out
}

// resolveAllowedOrigins picks the allowlist source: explicit Config.AllowedOrigins
// wins; otherwise falls back to the MOKU_ALLOWED_ORIGINS env var.
func resolveAllowedOrigins(cfg Config) []string {
	if len(cfg.AllowedOrigins) > 0 {
		return cfg.AllowedOrigins
	}
	return parseAllowedOrigins(os.Getenv(allowedOriginsEnvVar))
}

// isOriginAllowed reports whether an Origin header value should be allowed
// against the configured allowlist. Rules:
//   - empty allowlist or allowlist containing "*" → permissive (dev default)
//   - empty origin → allowed (non-browser client, e.g. curl or server-side Go)
//   - otherwise → normalized match on scheme+host (case-insensitive) with exact port
func isOriginAllowed(origin string, allowlist []string) bool {
	if isPermissiveAllowlist(allowlist) {
		return true
	}
	if origin == "" {
		return true
	}
	norm, ok := normalizeOriginForCompare(origin)
	if !ok {
		return false
	}
	for _, entry := range allowlist {
		if entry == "*" {
			return true
		}
		candidate, ok := normalizeOriginForCompare(entry)
		if !ok {
			continue
		}
		if candidate == norm {
			return true
		}
	}
	return false
}

func isPermissiveAllowlist(allowlist []string) bool {
	return len(allowlist) == 0 || slices.Contains(allowlist, "*")
}

// normalizeOriginForCompare lowercases scheme+host and keeps port exact.
// Returns false if the origin cannot be parsed into a scheme+host pair.
func normalizeOriginForCompare(origin string) (string, bool) {
	u, err := url.Parse(strings.TrimSuffix(strings.TrimSpace(origin), "/"))
	if err != nil {
		return "", false
	}
	if u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), true
}
