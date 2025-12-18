package assessor

import (
	"context"

	"github.com/raysh454/moku/internal/webclient"
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
	// ScoreHTML evaluates raw HTML bytes.
	// The opts parameter can request locations or a lightweight pass.
	ScoreHTML(ctx context.Context, html []byte, source string) (*ScoreResult, error)

	// ScoreResponse evaluates an already-fetched response (no network).
	ScoreResponse(ctx context.Context, resp *webclient.Response) (*ScoreResult, error)

	// Close releases any resources held by the assessor.
	Close() error
}
