package assessor

import (
	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/utils"
)

func NewSecurityDiff(
	url string,
	baseVersionID, headVersionID string,
	baseSnapshotID, headSnapshotID string,
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
		BaseVersionID:  baseVersionID,
		HeadVersionID:  headVersionID,
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

func NewSecurityDiffOverview(diffs []*SecurityDiff) *SecurityDiffOverview {
    ov := &SecurityDiffOverview{
        Entries: make([]SecurityDiffOverviewEntry, 0, len(diffs)),
    }

    if len(diffs) == 0 {
        return ov
    }

    // You can copy version IDs from the first diff if needed
    ov.BaseVersionID = diffs[0].BaseVersionID
    ov.HeadVersionID = diffs[0].HeadVersionID


    for _, d := range diffs {
        if d == nil {
            continue
        }

        entry := SecurityDiffOverviewEntry{
            FilePath:             d.FilePath,
            BaseSnapshotID:       d.BaseSnapshotID,
            HeadSnapshotID:       d.HeadSnapshotID,
            ScoreBase:            d.ScoreBase,
            ScoreHead:            d.ScoreHead,
            ScoreDelta:           d.ScoreDelta,
            AttackSurfaceChanged: d.AttackSurfaceChanged,
            NumAttackSurfaceChanges: len(d.AttackSurfaceChanges),
            Regressed:            d.ScoreDelta > 0,
        }

        ov.Entries = append(ov.Entries, entry)
    }

    return ov
}
