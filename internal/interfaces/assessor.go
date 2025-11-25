package interfaces

import (
	"context"

	"github.com/raysh454/moku/internal/model"
)


// Assessor is the minimal cross-package contract for scoring HTML content.
// Implementations receive HTML bytes (or an already-fetched model.Response)
// and return a ScoreResult. The Assessor does NOT perform network I/O.
//
// Assessor implementations SHOULD populate EvidenceItem.Locations (model.EvidenceLocation)
// for evidence items when RequestLocations==true and when the rule can produce
// a precise locator (CSS selector, XPath, byte/line offsets). This makes
// attribution deterministic and avoids fuzzy heuristics.
type Assessor interface {
	// ScoreHTML evaluates raw HTML bytes. 'source' is a user-provided identifier
	// for the content (e.g., URL) used for logging/evidence meta. The opts
	// parameter can request locations or a lightweight pass.
	ScoreHTML(ctx context.Context, html []byte, source string, opts model.ScoreOptions) (*model.ScoreResult, error)

	// ScoreResponse evaluates an already-fetched response (no network).
	ScoreResponse(ctx context.Context, resp *model.Response, opts model.ScoreOptions) (*model.ScoreResult, error)

	// ExtractEvidence is an optional helper that returns evidence items only
	// (useful for pre-extraction / fast attribution workflows). Implementations
	// may return an error ErrNotImplemented if they don't support this fast path.
	ExtractEvidence(ctx context.Context, html []byte, opts model.ScoreOptions) ([]model.EvidenceItem, error)

	// Close releases any resources held by the assessor.
	Close() error
}
