package webclient

import (
	"fmt"
	"net/http"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/raysh454/moku/internal/interfaces"
)

// RegisterDefaultBackends registers the default nethttp and chromedp backends.
// Call this from init() or early in main() to make backends available to NewWebClient.
func RegisterDefaultBackends() {
	// Register nethttp backend
	RegisterBackend("nethttp", func(cfg map[string]interface{}, logger interfaces.Logger) (interfaces.WebClient, error) {
		timeout := 30 * time.Second
		if t, ok := cfg["timeout"].(time.Duration); ok {
			timeout = t
		} else if t, ok := cfg["timeout"].(int); ok {
			timeout = time.Duration(t) * time.Second
		}

		client := &http.Client{
			Timeout: timeout,
		}

		if logger != nil {
			logger.Debug("created nethttp webclient", interfaces.Field{Key: "timeout", Value: timeout.String()})
		}

		return NewNetHTTPClient(client), nil
	})

	// Register chromedp backend
	RegisterBackend("chromedp", func(cfg map[string]interface{}, logger interfaces.Logger) (interfaces.WebClient, error) {
		idleAfter := 2 * time.Second
		if t, ok := cfg["idle_after"].(time.Duration); ok {
			idleAfter = t
		} else if t, ok := cfg["idle_after"].(int); ok {
			idleAfter = time.Duration(t) * time.Second
		}

		var opts []chromedp.ExecAllocatorOption
		if headless, ok := cfg["headless"].(bool); ok && !headless {
			// If headless is explicitly false, add option to show browser
			opts = append(opts, chromedp.Flag("headless", false))
		}

		client, err := NewChromeDPClient(idleAfter, opts...)
		if err != nil {
			return nil, fmt.Errorf("create chromedp client: %w", err)
		}

		if logger != nil {
			logger.Debug("created chromedp webclient", interfaces.Field{Key: "idle_after", Value: idleAfter.String()})
		}

		return client, nil
	})
}
