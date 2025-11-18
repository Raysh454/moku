package webclient

import (
	"fmt"
	"sync"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
)

// BackendFactory is a function that creates a WebClient instance.
// It receives the application config and logger and returns a WebClient or error.
type BackendFactory func(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error)

var (
	mu       sync.RWMutex
	backends = make(map[string]BackendFactory)
)

// RegisterBackend registers a webclient backend factory with the given name.
// This allows the application to register different implementations (nethttp, chromedp, etc.)
// at startup time without hard-coding dependencies in the factory package.
func RegisterBackend(name string, factory BackendFactory) {
	mu.Lock()
	defer mu.Unlock()
	backends[name] = factory
}

// NewWebClient creates a WebClient using the registered backend specified in cfg.
// Returns an error if no backend is registered or if the factory fails.
func NewWebClient(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}

	backendName := cfg.WebClientBackend
	if backendName == "" {
		backendName = "nethttp" // default backend
	}

	mu.RLock()
	factory, ok := backends[backendName]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("webclient backend %q not registered", backendName)
	}

	return factory(cfg, logger)
}
