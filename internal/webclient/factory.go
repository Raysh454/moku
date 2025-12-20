package webclient

import (
	"fmt"

	"github.com/raysh454/moku/internal/logging"
)

// NewWebClient constructs the configured WebClient backend based on cfg.GetWebClientBackend().
// It returns an error if the backend is not supported or if construction fails.
func NewWebClient(cfg Config, logger logging.Logger) (WebClient, error) {
	backend := cfg.Client
	if backend == "" {
		backend = "nethttp"
	}

	switch backend {
	case "nethttp":
		return NewNetHTTPClient(cfg, logger, nil)
	case "chromedp":
		return NewChromedpClient(cfg, logger)
	default:
		return nil, fmt.Errorf("unknown webclient backend %q (supported: nethttp, chromedp)", backend)
	}
}
