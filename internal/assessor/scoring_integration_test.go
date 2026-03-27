package assessor_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker/models"
)

func newTestAssessor(t *testing.T, rules []assessor.Rule) assessor.Assessor {
	t.Helper()
	cfg := &assessor.Config{
		ScoringVersion:    "v0.3.0",
		DefaultConfidence: 0.1,
	}
	logger := logging.NewStdoutLogger("integration-test")
	a, err := assessor.NewHeuristicsAssessor(cfg, rules, logger)
	if err != nil {
		t.Fatalf("NewHeuristicsAssessor error: %v", err)
	}
	return a
}

func TestScoreSnapshot_MissingHeadersOnly_DoesNotSaturate(t *testing.T) {
	t.Parallel()

	a := newTestAssessor(t, []assessor.Rule{})
	defer a.Close()

	html := []byte(`<html><head><title>Plain Page</title></head><body><p>Hello world</p></body></html>`)
	snapshot := &models.Snapshot{
		ID:         "snap-headers",
		URL:        "http://example.com/page",
		StatusCode: 200,
		Headers:    map[string][]string{},
		Body:       html,
	}

	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v-test")
	if err != nil {
		t.Fatalf("ScoreSnapshot error: %v", err)
	}

	if res.Normalized > 50 {
		t.Errorf("a plain page with only missing headers should score <=50, got %d (Score=%v)", res.Normalized, res.Score)
	}
	if res.Normalized < 5 {
		t.Errorf("missing headers should contribute some score, got %d", res.Normalized)
	}
}

func TestScoreSnapshot_CriticalPage_ScoresHigherThanPlain(t *testing.T) {
	t.Parallel()

	rules := assessor.DefaultRules()
	a := newTestAssessor(t, rules)
	defer a.Close()

	plainHTML := []byte(`<html><body><p>Hello</p></body></html>`)
	criticalHTML := []byte(`<html><body>
		<form action="/admin/upload">
			<input type="file" name="payload">
			<input type="password" name="pass">
			<input type="hidden" name="debug" value="1">
		</form>
		<script>var api_key = "sk-secret-12345";</script>
		<a href="javascript:void(0)">click</a>
	</body></html>`)

	plainSnap := &models.Snapshot{ID: "snap-plain", URL: "http://example.com/plain", StatusCode: 200, Headers: map[string][]string{}, Body: plainHTML}
	criticalSnap := &models.Snapshot{ID: "snap-critical", URL: "http://example.com/admin?debug=1&id=123", StatusCode: 200, Headers: map[string][]string{}, Body: criticalHTML}

	plainRes, _ := a.ScoreSnapshot(context.Background(), plainSnap, "v1")
	criticalRes, _ := a.ScoreSnapshot(context.Background(), criticalSnap, "v1")

	if criticalRes.Score <= plainRes.Score {
		t.Errorf("critical page should score higher than plain: critical=%v plain=%v", criticalRes.Score, plainRes.Score)
	}
	if criticalRes.Normalized < 40 {
		t.Errorf("critical page with admin forms, file upload, secrets should score >=40, got %d", criticalRes.Normalized)
	}
}

func TestScoreSnapshot_UploadWithoutCSRF_ScoresHigherThanWithCSRF(t *testing.T) {
	t.Parallel()

	a := newTestAssessor(t, []assessor.Rule{})
	defer a.Close()

	withoutCSRF := []byte(`<html><body>
		<form action="/upload">
			<input type="file" name="doc">
			<input type="submit">
		</form>
	</body></html>`)

	withCSRF := []byte(`<html><body>
		<form action="/upload">
			<input type="file" name="doc">
			<input type="hidden" name="csrf_token" value="abc123">
			<input type="submit">
		</form>
	</body></html>`)

	snapNoCSRF := &models.Snapshot{ID: "snap-no-csrf", URL: "http://example.com/upload", StatusCode: 200, Headers: map[string][]string{}, Body: withoutCSRF}
	snapCSRF := &models.Snapshot{ID: "snap-csrf", URL: "http://example.com/upload", StatusCode: 200, Headers: map[string][]string{}, Body: withCSRF}

	resNoCSRF, _ := a.ScoreSnapshot(context.Background(), snapNoCSRF, "v1")
	resCSRF, _ := a.ScoreSnapshot(context.Background(), snapCSRF, "v1")

	if resNoCSRF.Score <= resCSRF.Score {
		t.Errorf("upload without CSRF should score higher: noCSRF=%v withCSRF=%v", resNoCSRF.Score, resCSRF.Score)
	}
}

func TestScoreSnapshot_ProducesCategoryScores(t *testing.T) {
	t.Parallel()

	a := newTestAssessor(t, []assessor.Rule{})
	defer a.Close()

	html := []byte(`<html><body><form><input type="password"></form></body></html>`)
	snapshot := &models.Snapshot{ID: "snap-cats", URL: "http://example.com/login", StatusCode: 200, Headers: map[string][]string{}, Body: html}

	res, err := a.ScoreSnapshot(context.Background(), snapshot, "v1")
	if err != nil {
		t.Fatalf("ScoreSnapshot error: %v", err)
	}

	if len(res.CategoryScores) == 0 {
		t.Fatal("expected CategoryScores to be populated")
	}

	foundHeaders := false
	for _, cs := range res.CategoryScores {
		if cs.Category == assessor.CategoryHeaders {
			foundHeaders = true
			if cs.Score <= 0 {
				t.Error("headers category should have non-zero score (missing security headers)")
			}
		}
		if cs.Score < 0 || cs.Score > 1 {
			t.Errorf("category %q score out of bounds: %v", cs.Category, cs.Score)
		}
	}
	if !foundHeaders {
		t.Error("expected CategoryHeaders in CategoryScores")
	}
}

func TestScoreSnapshot_DynamicConfidence_VariesWithContent(t *testing.T) {
	t.Parallel()

	a := newTestAssessor(t, assessor.DefaultRules())
	defer a.Close()

	smallSnap := &models.Snapshot{ID: "snap-tiny", URL: "http://example.com/", StatusCode: 200, Headers: map[string][]string{}, Body: []byte("<html></html>")}
	richSnap := &models.Snapshot{
		ID:         "snap-rich",
		URL:        "http://example.com/app",
		StatusCode: 200,
		Headers: map[string][]string{
			"set-cookie": {"session=abc; HttpOnly; Secure"},
		},
		Body: []byte(`<html><body>
			<form action="/login"><input type="password" name="pw"><input type="hidden" name="csrf" value="tok"></form>
			<script src="/app.js"></script>
			<script>console.log("debug")</script>
			<p>Some content that makes the body larger than 100 bytes to increase confidence scoring.</p>
		</body></html>`),
	}

	smallRes, _ := a.ScoreSnapshot(context.Background(), smallSnap, "v1")
	richRes, _ := a.ScoreSnapshot(context.Background(), richSnap, "v1")

	if richRes.Confidence <= smallRes.Confidence {
		t.Errorf("rich page should have higher confidence: rich=%v small=%v", richRes.Confidence, smallRes.Confidence)
	}
}

func TestScoreSnapshot_ScoreAlwaysInBounds(t *testing.T) {
	t.Parallel()

	a := newTestAssessor(t, assessor.DefaultRules())
	defer a.Close()

	htmls := [][]byte{
		[]byte(""),
		[]byte("<html></html>"),
		[]byte(`<html><body>
			<form action="/admin"><input type="file"><input type="password"></form>
			<script>var token = "secret123"</script>
			<a href="javascript:alert(1)">xss</a>
			<iframe src="http://evil.com"></iframe>
			<base href="http://evil.com">
			<!-- TODO: fix this -->
		</body></html>`),
	}

	for i, html := range htmls {
		snap := &models.Snapshot{ID: "snap-bounds", URL: "http://example.com/", StatusCode: 200, Headers: map[string][]string{}, Body: html}
		res, err := a.ScoreSnapshot(context.Background(), snap, "v1")
		if err != nil {
			t.Fatalf("case %d: ScoreSnapshot error: %v", i, err)
		}
		if res.Score < 0.0 || res.Score > 1.0 {
			t.Errorf("case %d: Score out of bounds: %v", i, res.Score)
		}
		if res.Normalized < 0 || res.Normalized > 100 {
			t.Errorf("case %d: Normalized out of bounds: %v", i, res.Normalized)
		}
	}
}

func TestDiffScores_CategoryDeltas_Computed(t *testing.T) {
	t.Parallel()

	base := &assessor.ScoreResult{
		Score:         0.2,
		RawFeatures:   map[string]float64{"csp_missing": 1},
		ContribByRule: map[string]float64{"csp_missing": 0.5},
		CategoryScores: []assessor.CategoryScoreEntry{
			{Category: assessor.CategoryHeaders, Score: 0.5},
			{Category: assessor.CategoryForms, Score: 0.0},
		},
	}
	head := &assessor.ScoreResult{
		Score:         0.4,
		RawFeatures:   map[string]float64{"csp_missing": 1, "has_file_upload": 1},
		ContribByRule: map[string]float64{"csp_missing": 0.5, "has_file_upload": 0.4},
		CategoryScores: []assessor.CategoryScoreEntry{
			{Category: assessor.CategoryHeaders, Score: 0.5},
			{Category: assessor.CategoryForms, Score: 0.3},
		},
	}

	diff := assessor.DiffScores(base, head)

	if diff.CategoryDeltas == nil {
		t.Fatal("expected CategoryDeltas to be non-nil")
	}
	if delta, ok := diff.CategoryDeltas[assessor.CategoryForms]; !ok || delta != 0.3 {
		t.Errorf("expected forms delta 0.3, got %v", delta)
	}
	if _, ok := diff.CategoryDeltas[assessor.CategoryHeaders]; ok {
		t.Error("headers delta should be absent (no change)")
	}
}
