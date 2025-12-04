package webclient

import (
	"context"
)

type WebClient interface {
	Do(ctx context.Context, req *Request) (*Response, error)

	// Get is a convenience method for simple GET requests
	Get(ctx context.Context, url string) (*Response, error)

	Close() error
}
