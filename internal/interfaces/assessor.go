package interfaces

import (
	"context"

	"github.com/raysh454/moku/internal/model"
)

// Assessor is the minimal cross-package contract for scoring HTML content.
// Implementations receive HTML bytes (or an already-fetched model.Response)
// and return a ScoreResult. The Assessor does NOT perform network I/O.
//
// Note: this interface intentionally references model.ScoreResult so callers
// and implementations agree on the canonical result type.
type Assessor interface {
	// ScoreHTML evaluates raw HTML bytes. 'source' is a user-provided identifier
	// for the content (e.g., URL or fixture name) used for logging/evidence meta.
	ScoreHTML(ctx context.Context, html []byte, source string) (*model.ScoreResult, error)

	// ScoreResponse evaluates an already-fetched response (no network).
	ScoreResponse(ctx context.Context, resp *model.Response) (*model.ScoreResult, error)

	// Close releases any resources held by the assessor.
	Close() error
}
