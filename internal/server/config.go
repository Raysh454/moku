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

	// AllowedOrigins is the CORS + WebSocket origin allowlist. When nil or
	// empty, the server falls back to the MOKU_ALLOWED_ORIGINS environment
	// variable (comma-separated). An empty result or a list containing "*"
	// preserves the permissive dev default.
	AllowedOrigins []string
}
