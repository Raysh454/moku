package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

const (
	// headerAPIToken carries the shared secret on authenticated requests.
	headerAPIToken = "X-Moku-Token"
	// queryAPITokenParam is the query-parameter fallback accepted ONLY on the
	// SSE events stream, where browser EventSource clients cannot set headers.
	queryAPITokenParam = "token"
	// apiTokenEnvVar is the environment fallback for the configured API token.
	apiTokenEnvVar = "MOKU_API_TOKEN"
	// sseEventsPath is the one route allowed to authenticate via query param.
	sseEventsPath = "/jobs/events"
	// swaggerPathPrefix is exempt from authentication (interactive docs).
	swaggerPathPrefix = "/swagger/"
)

// resolveAPIToken picks the token source: explicit Config.APIToken wins;
// otherwise it falls back to the MOKU_API_TOKEN environment variable. An empty
// result disables authentication (the middleware becomes a no-op).
func resolveAPIToken(cfg Config) string {
	if cfg.APIToken != "" {
		return cfg.APIToken
	}
	return os.Getenv(apiTokenEnvVar)
}

// authMiddleware enforces the shared-token policy. When no token is configured
// it passes every request through unchanged. Otherwise it requires a matching
// token in the X-Moku-Token header, with a ?token= query fallback on the SSE
// events stream only. OPTIONS preflight requests and /swagger/ paths are exempt.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		if isAuthExempt(r) {
			next.ServeHTTP(w, r)
			return
		}
		if tokensEqual(s.apiToken, presentedToken(r)) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

// isAuthExempt reports whether a request bypasses authentication: CORS preflight
// (OPTIONS) and the Swagger documentation UI.
func isAuthExempt(r *http.Request) bool {
	if r.Method == http.MethodOptions {
		return true
	}
	return strings.HasPrefix(r.URL.Path, swaggerPathPrefix)
}

// presentedToken extracts the caller-supplied token. The header is the primary
// source; the ?token= query parameter is honored only for the SSE events
// stream, which browser EventSource clients cannot authenticate via headers.
func presentedToken(r *http.Request) string {
	if header := r.Header.Get(headerAPIToken); header != "" {
		return header
	}
	if r.URL.Path == sseEventsPath {
		return r.URL.Query().Get(queryAPITokenParam)
	}
	return ""
}

// tokensEqual compares two tokens in constant time. Both values are hashed with
// SHA-256 first so the comparison length never leaks the configured token's
// length and equal-length inputs are guaranteed for ConstantTimeCompare.
func tokensEqual(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
