package assessor

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

type HeuristicsAssessor struct {
	cfg    *Config
	logger logging.Logger
}

func NewHeuristicsAssessor(cfg *Config, logger logging.Logger) (Assessor, error) {
	if cfg == nil {
		return nil, errors.New("assessor: nil config; please pass a valid Config")
	}
	if logger == nil {
		return nil, errors.New("assessor: nil logger; please pass a valid logging.Logger")
	}

	l := logger.With(logging.Field{Key: "component", Value: "heuristics-assessor"})

	inst := &HeuristicsAssessor{
		cfg:    cfg,
		logger: l,
	}

	l.Info("heuristics assessor constructed", logging.Field{Key: "scoring_version", Value: cfg.ScoringVersion})

	return inst, nil
}

func (h *HeuristicsAssessor) scoreUsingAttackSurface(snapshot *models.Snapshot, res *ScoreResult) (*attacksurface.AttackSurface, error) {

	at, err := attacksurface.BuildAttackSurfaceFromHTML(snapshot.URL, snapshot.ID, snapshot.StatusCode, snapshot.Headers, snapshot.Body)
	if err != nil {
		h.logger.Warn("failed to build attack surface from html",
			logging.Field{Key: "err", Value: err},
			logging.Field{Key: "snapshot_id", Value: snapshot.ID},
			logging.Field{Key: "url", Value: snapshot.URL},
		)
		return nil, err
	}

	if at != nil {
		feats := attacksurface.ComputeFeatures(at)
		res.RawFeatures = feats

		for name, val := range feats {
			w, ok := attacksurface.FeatureWeights[name]
			if !ok || val == 0 {
				continue
			}
			contrib := w * val

			locs := []EvidenceLocation{}
			if h.cfg.ScoreOpts.RequestLocations {
				locs = buildFeatureLocations(name, at)
			}

			res.Evidence = append(res.Evidence, EvidenceItem{
				Key:          name,
				RuleID:       name,
				Severity:     attacksurface.SeverityForFeature(name),
				Description:  attacksurface.DescribeFeature(name),
				Value:        val,
				Contribution: contrib,
				Locations:    locs,
			})
			res.ContribByRule[name] += contrib
		}
	}

	return at, nil
}

func (h *HeuristicsAssessor) ScoreSnapshot(ctx context.Context, snapshot *models.Snapshot, versionID string) (*ScoreResult, error) {
	if h == nil || h.logger == nil {
		return nil, errors.New("heuristics-assessor: nil instance or logger")
	}

	res := newScoreResult(snapshot, versionID, h.cfg)

	as, err := h.scoreUsingAttackSurface(snapshot, res)
	if err != nil {
		h.logger.Warn("error during attack surface scoring", logging.Field{Key: "err", Value: err}, logging.Field{Key: "snapshot_id", Value: snapshot.ID})
	}
	res.AttackSurface = as

	h.computeScore(res)

	return res, nil
}

func (*HeuristicsAssessor) computeScore(res *ScoreResult) {
	var totalContrib float64
	for _, contrib := range res.ContribByRule {
		totalContrib += contrib
	}
	res.Score = normalizeScore(totalContrib)
	res.Normalized = int(res.Score * 100.0)
}

func (h *HeuristicsAssessor) Close() error {
	if h == nil || h.logger == nil {
		return nil
	}
	h.logger.Info("heuristics-assessor: closed")
	return nil
}

func newScoreResult(snapshot *models.Snapshot, versionID string, cfg *Config) *ScoreResult {
	return &ScoreResult{
		Score:         0.0,
		SnapshotID:    snapshot.ID,
		VersionID:     versionID,
		Normalized:    0,
		Confidence:    cfg.DefaultConfidence,
		Version:       cfg.ScoringVersion,
		Evidence:      []EvidenceItem{},
		RawFeatures:   map[string]float64{},
		ContribByRule: map[string]float64{},
		Meta:          map[string]any{"snapshot_id": snapshot.ID, "url": snapshot.URL},
		Timestamp:     time.Now().UTC(),
	}
}

func buildFeatureLocations(featureName string, as *attacksurface.AttackSurface) []EvidenceLocation {
	if as == nil {
		return nil
	}

	switch featureName {

	case "csp_missing", "csp_unsafe_inline", "csp_unsafe_eval":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "content-security-policy",
			SnapshotID: as.SnapshotID,
		}}

	case "xfo_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "x-frame-options",
			SnapshotID: as.SnapshotID,
		}}

	case "xcto_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "x-content-type-options",
			SnapshotID: as.SnapshotID,
		}}

	case "hsts_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "strict-transport-security",
			SnapshotID: as.SnapshotID,
		}}

	case "referrer_policy_missing":
		return []EvidenceLocation{{
			Type:       "header",
			HeaderName: "referrer-policy",
			SnapshotID: as.SnapshotID,
		}}

	case "has_session_cookie_no_httponly":
		for _, c := range as.Cookies {
			if strings.Contains(strings.ToLower(c.Name), "session") && !c.HttpOnly {
				return []EvidenceLocation{{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				}}
			}
		}
		return nil

	case "num_cookies_missing_httponly":
		var locs []EvidenceLocation
		for _, c := range as.Cookies {
			if !c.HttpOnly {
				locs = append(locs, EvidenceLocation{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				})
			}
		}
		return locs

	case "num_cookies_missing_secure":
		var locs []EvidenceLocation
		for _, c := range as.Cookies {
			if !c.Secure {
				locs = append(locs, EvidenceLocation{
					Type:       "cookie",
					CookieName: c.Name,
					SnapshotID: as.SnapshotID,
				})
			}
		}
		return locs

	case "has_file_upload", "num_file_inputs":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			for _, in := range f.Inputs {
				if strings.EqualFold(in.Type, "file") {
					fIdx := f.DOMIndex
					iIdx := in.DOMIndex
					locs = append(locs, EvidenceLocation{
						Type:           "input",
						SnapshotID:     as.SnapshotID,
						ParentDOMIndex: &fIdx,
						DOMIndex:       &iIdx,
					})
				}
			}
		}
		return locs

	case "has_password_input", "num_password_inputs":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			for _, in := range f.Inputs {
				if strings.EqualFold(in.Type, "password") {
					fIdx := f.DOMIndex
					iIdx := in.DOMIndex
					locs = append(locs, EvidenceLocation{
						Type:           "input",
						SnapshotID:     as.SnapshotID,
						ParentDOMIndex: &fIdx,
						DOMIndex:       &iIdx,
					})
				}
			}
		}
		return locs

	case "has_admin_form", "has_auth_form", "has_upload_form":
		var locs []EvidenceLocation
		for _, f := range as.Forms {
			fIdx := f.DOMIndex
			locs = append(locs, EvidenceLocation{
				Type:       "form",
				SnapshotID: as.SnapshotID,
				DOMIndex:   &fIdx,
			})
		}
		return locs

	case "has_admin_param", "has_upload_param", "has_debug_param", "has_id_param", "num_suspicious_params":
		return []EvidenceLocation{{
			Type:       "param",
			SnapshotID: as.SnapshotID,
		}}

	default:
		return nil
	}
}

func normalizeScore(rawScore float64) float64 {
	if rawScore <= 0 {
		return 0.0
	}
	k := 3.0
	return 1.0 - math.Exp(-k*rawScore)
}
