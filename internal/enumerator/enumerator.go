package enumerator

import (
	"context"

	"github.com/raysh454/moku/internal/utils"
)

type Enumerator interface {
	Enumerate(ctx context.Context, target string, cb utils.ProgressCallback) ([]string, error)
}
