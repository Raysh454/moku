package enumerator

import "context"

type Enumerator interface {
	Enumerate(ctx context.Context, target string) ([]string, error)
}
