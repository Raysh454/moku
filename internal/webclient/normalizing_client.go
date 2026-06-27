package webclient

import (
	"context"

	"github.com/raysh454/moku/internal/htmlnorm"
	"github.com/raysh454/moku/internal/logging"
)

// NormalizingClient is a Decorator that runs a Normalizer over the rendered HTML
// of every response before returning it, so downstream snapshots and diffs see
// canonical, low-noise content. Normalization is best-effort: if it fails, the
// raw body is returned rather than dropping the fetch.
type NormalizingClient struct {
	inner      WebClient
	normalizer *htmlnorm.Normalizer
	logger     logging.Logger
}

// NewNormalizingClient wraps inner so its response bodies are normalized.
func NewNormalizingClient(inner WebClient, normalizer *htmlnorm.Normalizer, logger logging.Logger) WebClient {
	return &NormalizingClient{
		inner:      inner,
		normalizer: normalizer,
		logger:     logger.With(logging.Field{Key: "decorator", Value: "normalizing"}),
	}
}

// Do delegates the fetch and normalizes the body of a successful response.
func (c *NormalizingClient) Do(ctx context.Context, req *Request) (*Response, error) {
	resp, err := c.inner.Do(ctx, req)
	if err != nil || resp == nil {
		return resp, err
	}

	normalized, normErr := c.normalizer.Normalize(resp.Body)
	if normErr != nil {
		c.logger.Warn("normalization failed; returning raw body",
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "error", Value: normErr.Error()})
		return resp, nil
	}

	out := *resp
	out.Body = normalized
	return &out, nil
}

// Get is a convenience method for simple GET requests.
func (c *NormalizingClient) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, &Request{Method: "GET", URL: url})
}

// Close delegates to the wrapped client.
func (c *NormalizingClient) Close() error {
	return c.inner.Close()
}
