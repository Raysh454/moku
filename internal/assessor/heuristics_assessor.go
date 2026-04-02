package assessor

import (
	"context"
	"errors"
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

func (h *HeuristicsAssessor) ScoreSnapshot(ctx context.Context, snapshot *models.Snapshot, versionID string) (*ScoreResult, error) {
	if h == nil || h.logger == nil {
		return nil, errors.New("heuristics-assessor: nil instance or logger")
	}

	res := newScoreResult(snapshot, versionID, h.cfg)

	as, err := attacksurface.BuildAttackSurfaceFromHTML(snapshot.URL, snapshot.ID, snapshot.StatusCode, snapshot.Headers, snapshot.Body)
	if err != nil {
		h.logger.Warn("failed to build attack surface from html",
			logging.Field{Key: "err", Value: err},
			logging.Field{Key: "snapshot_id", Value: snapshot.ID},
			logging.Field{Key: "url", Value: snapshot.URL},
		)
		return res, nil
	}

	res.AttackSurface = as

	if as != nil {
		res.ExposureScore = attacksurface.ComputeExposureScore(as, &h.cfg.Saturation)
		res.HardeningScore = attacksurface.ComputeHardeningScore(as.Headers)
		res.PostureScore = ComputePostureScore(res.ExposureScore, res.HardeningScore)
		res.Score = res.PostureScore
		res.Normalized = int(res.Score * 100.0)

		res.Evidence = buildEvidenceFromAttackSurface(as, h.cfg.ScoreOpts.RequestLocations)
	}

	return res, nil
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
		Score:      0.0,
		SnapshotID: snapshot.ID,
		VersionID:  versionID,
		Normalized: 0,
		Confidence: cfg.DefaultConfidence,
		Version:    cfg.ScoringVersion,
		Evidence:   []EvidenceItem{},
		Meta:       map[string]any{"snapshot_id": snapshot.ID, "url": snapshot.URL},
		Timestamp:  time.Now().UTC(),
	}
}

func buildEvidenceFromAttackSurface(as *attacksurface.AttackSurface, withLocations bool) []EvidenceItem {
	var evidence []EvidenceItem

	// Forms
	for _, form := range as.Forms {
		action := strings.ToLower(form.Action)
		var key, desc, severity string
		switch {
		case strings.Contains(action, "admin") || strings.Contains(action, "/admin"):
			key, desc, severity = "admin_form", "Page contains forms targeting admin-like paths", "high"
		case strings.Contains(action, "login") || strings.Contains(action, "signin") || strings.Contains(action, "auth"):
			key, desc, severity = "auth_form", "Page contains login/authentication forms", "medium"
		case strings.Contains(action, "upload") || strings.Contains(action, "/upload") || strings.Contains(action, "file"):
			key, desc, severity = "upload_form", "Page contains forms targeting upload endpoints", "high"
		default:
			key, desc, severity = "form", "HTML form on the page", "low"
		}
		score := attacksurface.ElementScores["form_"+key[0:0]]
		if s, ok := attacksurface.ElementScores[key]; ok {
			score = s
		}
		ev := EvidenceItem{Key: key, RuleID: key, Severity: severity, Description: desc, Contribution: score}
		if withLocations {
			idx := form.DOMIndex
			ev.Locations = []EvidenceLocation{{Type: "form", SnapshotID: as.SnapshotID, DOMIndex: &idx}}
		}
		evidence = append(evidence, ev)
	}

	// Inputs
	for _, form := range as.Forms {
		for _, in := range form.Inputs {
			t := strings.ToLower(in.Type)
			var key, desc, severity string
			var score float64
			switch t {
			case "file":
				key, desc, severity = "input_file", "Page exposes file upload functionality", "high"
				score = attacksurface.ElementScores["input_file"]
			case "password":
				key, desc, severity = "input_password", "Page contains password input fields", "high"
				score = attacksurface.ElementScores["input_password"]
			default:
				continue
			}
			ev := EvidenceItem{Key: key, RuleID: key, Severity: severity, Description: desc, Contribution: score}
			if withLocations {
				fIdx := form.DOMIndex
				iIdx := in.DOMIndex
				ev.Locations = []EvidenceLocation{{Type: "input", SnapshotID: as.SnapshotID, ParentDOMIndex: &fIdx, DOMIndex: &iIdx}}
			}
			evidence = append(evidence, ev)
		}
	}

	// Cookies
	for _, c := range as.Cookies {
		if strings.Contains(strings.ToLower(c.Name), "session") && !c.HttpOnly {
			ev := EvidenceItem{
				Key: "session_cookie_no_httponly", RuleID: "session_cookie_no_httponly",
				Severity: "high", Description: "Session-like cookie missing HttpOnly flag",
				Contribution: attacksurface.ElementScores["cookie_session"],
			}
			if withLocations {
				ev.Locations = []EvidenceLocation{{Type: "cookie", CookieName: c.Name, SnapshotID: as.SnapshotID}}
			}
			evidence = append(evidence, ev)
		}
		if !c.HttpOnly {
			ev := EvidenceItem{
				Key: "cookie_no_httponly", RuleID: "cookie_no_httponly",
				Severity: "medium", Description: "Cookie missing the HttpOnly flag",
				Contribution: attacksurface.ElementScores["cookie_no_httponly"],
			}
			if withLocations {
				ev.Locations = []EvidenceLocation{{Type: "cookie", CookieName: c.Name, SnapshotID: as.SnapshotID}}
			}
			evidence = append(evidence, ev)
		}
		if !c.Secure {
			ev := EvidenceItem{
				Key: "cookie_no_secure", RuleID: "cookie_no_secure",
				Severity: "medium", Description: "Cookie missing the Secure flag",
				Contribution: attacksurface.ElementScores["cookie_no_secure"],
			}
			if withLocations {
				ev.Locations = []EvidenceLocation{{Type: "cookie", CookieName: c.Name, SnapshotID: as.SnapshotID}}
			}
			evidence = append(evidence, ev)
		}
	}

	// Security headers
	headerChecks := []struct {
		name     string
		key      string
		desc     string
		severity string
	}{
		{"content-security-policy", "csp_missing", "Content-Security-Policy header is missing", "high"},
		{"strict-transport-security", "hsts_missing", "Strict-Transport-Security header is missing", "medium"},
		{"x-frame-options", "xfo_missing", "X-Frame-Options header is missing", "medium"},
		{"x-content-type-options", "xcto_missing", "X-Content-Type-Options header is missing", "low"},
		{"referrer-policy", "referrer_policy_missing", "Referrer-Policy header is missing", "low"},
	}
	for _, check := range headerChecks {
		if _, ok := as.Headers[check.name]; !ok {
			ev := EvidenceItem{Key: check.key, RuleID: check.key, Severity: check.severity, Description: check.desc}
			if withLocations {
				ev.Locations = []EvidenceLocation{{Type: "header", HeaderName: check.name, SnapshotID: as.SnapshotID}}
			}
			evidence = append(evidence, ev)
		}
	}

	return evidence
}
