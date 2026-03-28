package assessor

import (
	"context"

	"github.com/raysh454/moku/internal/tracker/models"
)

// Assessor is the minimal cross-package contract for scoring HTML content.
// Implementations receive HTML bytes (or an already-fetched model.Response)
// and return a ScoreResult. The Assessor does NOT perform network I/O.
//
// Assessor implementations SHOULD populate EvidenceItem.Locations (model.EvidenceLocation)
// for evidence items when RequestLocations==true and when attack surface features can produce
// a precise locator (DOM index, XPath, byte/line offsets, header/cookie names). This makes
// attribution deterministic and avoids fuzzy heuristics.
type Assessor interface {
	// ScoreSnapshot scores the provided snapshot using attack surface analysis.
	// The configuration can request locations or a lightweight pass.
	ScoreSnapshot(ctx context.Context, snapshot *models.Snapshot, versionID string) (*ScoreResult, error)

	// Close releases any resources held by the assessor.
	Close() error
}
