package webclient

import (
	"fmt"
	"sync"

	"github.com/raysh454/moku/internal/interfaces"
)

// BackendFactory is a function that creates a WebClient backend from configuration.
type BackendFactory func(cfg map[string]interface{}, logger interfaces.Logger) (interfaces.WebClient, error)

var (
	mu        sync.RWMutex
	backends  = make(map[string]BackendFactory)
)

// RegisterBackend registers a named backend factory. Call this from init() or main()
// to register concrete implementations like "nethttp" or "chromedp".
func RegisterBackend(name string, factory BackendFactory) {
	mu.Lock()
	defer mu.Unlock()
	backends[name] = factory
}

// NewWebClient creates a WebClient instance using a registered backend.
// cfg should contain a "backend" key specifying which backend to use.
// Returns an error if no backend is registered with that name.
func NewWebClient(cfg map[string]interface{}, logger interfaces.Logger) (interfaces.WebClient, error) {
	backendName, ok := cfg["backend"].(string)
	if !ok || backendName == "" {
		return nil, fmt.Errorf("webclient config missing 'backend' key or value is not a string")
	}

	mu.RLock()
	factory, exists := backends[backendName]
	mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("webclient backend %q not registered; available backends: %v", backendName, availableBackends())
	}

	return factory(cfg, logger)
}

// availableBackends returns a list of registered backend names for error messages.
func availableBackends() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	return names
}
