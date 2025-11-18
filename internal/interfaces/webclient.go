package interfaces

import (
	"context"
	"github.com/raysh454/moku/internal/model"
)

type WebClient interface {
	Do(ctx context.Context, req *model.Request) (*model.Response, error)

	// Get is a convenience method for simple GET requests
	Get(ctx context.Context, url string) (*model.Response, error)

	Close() error
}

