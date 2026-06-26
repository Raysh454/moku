package webclient

import (
	"context"

	"github.com/raysh454/moku/internal/logging"
)

// BrowserHeaderClient is a Decorator that adds realistic browser navigation
// headers (User-Agent, Accept, Accept-Language) to every request before
// delegating to the wrapped WebClient, without overriding any header the caller
// already set. It is the cheapest disguise tier: useful over net/http when a
// target only inspects headers, but it does NOT change the TLS/HTTP2
// fingerprint, so it will not fool fingerprint-based bot management on its own.
type BrowserHeaderClient struct {
	inner  WebClient
	logger logging.Logger
}

// NewBrowserHeaderClient wraps inner so its requests carry browser headers.
func NewBrowserHeaderClient(inner WebClient, logger logging.Logger) WebClient {
	return &BrowserHeaderClient{
		inner:  inner,
		logger: logger.With(logging.Field{Key: "decorator", Value: "browser-headers"}),
	}
}

// Do decorates the request with browser headers and delegates. The caller's
// request and header map are left untouched (a clone is decorated instead).
func (c *BrowserHeaderClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return c.inner.Do(ctx, req)
	}
	decorated := *req
	decorated.Headers = ensureBrowserHeaders(req.Headers.Clone())
	return c.inner.Do(ctx, &decorated)
}

// Get is a convenience method for simple GET requests.
func (c *BrowserHeaderClient) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, &Request{Method: "GET", URL: url})
}

// Close delegates to the wrapped client.
func (c *BrowserHeaderClient) Close() error {
	return c.inner.Close()
}
