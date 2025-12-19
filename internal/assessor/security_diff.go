package assessor

import (
	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/utils"
)

func NewSecurityDiff(
	url string,
	baseSnapshotID string,
	headSnapshotID string,
	baseScore *ScoreResult,
	headScore *ScoreResult,
	baseAS *attacksurface.AttackSurface,
	headAS *attacksurface.AttackSurface,
) (*SecurityDiff, error) {
	scoreDiff := DiffScores(baseScore, headScore)
	asChanges := attacksurface.DiffAttackSurfaces(baseAS, headAS)

	urlTools, err := utils.NewURLTools(url)
	if err != nil {
		return nil, err
	}
	filepath := urlTools.GetPath()

	return &SecurityDiff{
		FilePath:       filepath,
		BaseSnapshotID: baseSnapshotID,
		HeadSnapshotID: headSnapshotID,

		ScoreBase:     scoreDiff.ScoreBase,
		ScoreHead:     scoreDiff.ScoreHead,
		ScoreDelta:    scoreDiff.ScoreDelta,
		FeatureDeltas: scoreDiff.FeatureDeltas,
		RuleDeltas:    scoreDiff.RuleDeltas,

		AttackSurfaceChanged: len(asChanges) > 0,
		AttackSurfaceChanges: asChanges,
	}, nil
}
