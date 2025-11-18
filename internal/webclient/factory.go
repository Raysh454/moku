package webclient

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
)

// BackendConstructor constructs an interfaces.WebClient given the config and logger.
type BackendConstructor func(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error)

var (
	mu        sync.RWMutex
	registry  = map[string]BackendConstructor{}
)

// RegisterBackend registers a named backend constructor. Name is lower-cased
// internally. Calling RegisterBackend with the same name overwrites the previous
// constructor.
func RegisterBackend(name string, ctor BackendConstructor) {
	if name == "" || ctor == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(name)] = ctor
}

// NewWebClient constructs the configured WebClient backend. It returns an error
// if the named backend has not been registered.
func NewWebClient(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.WebClientBackend))
	if backend == "" {
		backend = "nethttp"
	}

	mu.RLock()
	ctor, ok := registry[backend]
	mu.RUnlock()
	if !ok || ctor == nil {
		return nil, fmt.Errorf("webclient backend %q not registered: available backends=%v", backend, ListBackends())
	}

	wc, err := ctor(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to construct webclient backend %q: %w", backend, err)
	}
	if wc == nil {
		return nil, errors.New("webclient constructor returned nil")
	}
	return wc, nil
}

// ListBackends returns the list of registered backend names.
func ListBackends() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
