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
}
