package webclient

import (
	"context"
	"github.com/raysh454/moku/internal/model"
)

type WebClient interface {
	Do(ctx context.Context, req *model.Request) (*model.Response, error)

	Close() error
}
