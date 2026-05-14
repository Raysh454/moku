package enumerator

import (
	"context"
	"errors"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
)

// ErrRootUnreachable is returned by an enumerator when the target root page
// could not be reached (DNS/network etc). When composite encounters this
// error it should propagate it as a fatal condition.
var ErrRootUnreachable = errors.New("root unreachable")

type Composite struct {
	enumerators []Enumerator
	logger      logging.Logger
}

func NewComposite(enumerators []Enumerator, logger logging.Logger) *Composite {
	return &Composite{enumerators: enumerators, logger: logger}
}

func (c *Composite) Enumerate(ctx context.Context, target string, cb utils.ProgressCallback) ([]string, error) {
	seen := make(map[string]struct{})
	var results []string
	total := len(c.enumerators)

	for i, e := range c.enumerators {
		if ctx.Err() != nil {
			break
		}

		urls, err := e.Enumerate(ctx, target, nil)
		if err != nil {
			// Propagate fatal root-unreachable errors immediately so the job can
			// be marked as failed even if other enumerators produce results.
			if errors.Is(err, ErrRootUnreachable) {
				if c.logger != nil {
					c.logger.Warn("enumerator failed (root unreachable)",
						logging.Field{Key: "index", Value: i},
						logging.Field{Key: "error", Value: err})
				}
				return nil, err
			}

			if c.logger != nil {
				c.logger.Warn("enumerator failed, continuing",
					logging.Field{Key: "index", Value: i},
					logging.Field{Key: "error", Value: err})
			}
			continue
		}

		for _, u := range urls {
			if _, exists := seen[u]; !exists {
				seen[u] = struct{}{}
				results = append(results, u)
			}
		}

		if cb != nil {
			cb(i+1, 0, total)
		}
	}

	return results, nil
}
