package server

import (
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
)

type Config struct {
	// HTTP listen address, e.g. ":8080"
	ListenAddr string

	// Application-level config
	AppConfig *app.Config

	// Logger to use. If nil, server will construct a default no-op logger
	Logger logging.Logger

	// AllowedOrigins is the CORS origin allowlist. When nil or
	// empty, the server falls back to the MOKU_ALLOWED_ORIGINS environment
	// variable (comma-separated). An empty result or a list containing "*"
	// preserves the permissive dev default.
	AllowedOrigins []string

	// APIToken is the shared secret required on every request via the
	// X-Moku-Token header (or the ?token= query parameter on the SSE stream).
	// When empty, the server falls back to the MOKU_API_TOKEN environment
	// variable. When both are empty the auth middleware is a no-op and no
	// authentication is enforced — set it whenever the server is reachable
	// beyond loopback.
	APIToken string
}
