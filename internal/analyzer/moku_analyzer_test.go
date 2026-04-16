package analyzer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/webclient"
)

// newMokuForTest constructs a Moku analyzer with the supplied fakes and
// registers a t.Cleanup to close it after the test.
func newMokuForTest(t *testing.T, wc webclient.WebClient, as assessor.Assessor) analyzer.Analyzer {
	t.Helper()
	a, err := analyzer.NewAnalyzer(analyzer.Config{
		Backend: analyzer.BackendMoku,
		DefaultPoll: analyzer.PollOptions{
			Timeout:  2 * time.Second,
			Interval: 20 * time.Millisecond,
		},
		Moku: analyzer.MokuConfig{
			DefaultProfile: analyzer.ProfileBalanced,
			JobRetention:   1 * time.Minute,
		},
	}, analyzer.Dependencies{
		Logger:    noopLogger{},
		WebClient: wc,
		Assessor:  as,
	})
	if err != nil {
		t.Fatalf("NewAnalyzer: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// TestMokuAnalyzer_ScanAndWait_PopulatesFindingsFromEvidence asserts that the
// canned evidence item produced by the fake assessor surfaces as a Finding
// with matching Title, Severity, and Description — i.e. findingsFromScoreResult
// performs the mapping described in the plan.
func TestMokuAnalyzer_ScanAndWait_PopulatesFindingsFromEvidence(t *testing.T) {
	a := newMokuForTest(t, cannedHappyPathWebClient(), cannedHappyPathAssessor())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if result.Status != analyzer.StatusCompleted {
		t.Fatalf("Status = %q, want %q; error=%q", result.Status, analyzer.StatusCompleted, result.Error)
	}
	if got := len(result.Findings); got != 1 {
		t.Fatalf("len(Findings) = %d, want 1", got)
	}
	f := result.Findings[0]
	if f.Title == "" {
		t.Error("Finding.Title is empty; want humanized evidence key")
	}
	if f.Severity != analyzer.SeverityMedium {
		t.Errorf("Finding.Severity = %q, want %q", f.Severity, analyzer.SeverityMedium)
	}
	if f.Description == "" {
		t.Error("Finding.Description is empty; want evidence description")
	}
	if f.ID == "" {
		t.Error("Finding.ID is empty; want evidence RuleID")
	}
}

// TestMokuAnalyzer_ScanAndWait_SummaryCountsMatchFindings asserts the shape
// of ScanSummary: Total matches len(Findings) and per-severity counts sum to
// Total.
func TestMokuAnalyzer_ScanAndWait_SummaryCountsMatchFindings(t *testing.T) {
	multiSeverity := &fakeAssessor{
		scoreFunc: func(ctx context.Context, snapshot *models.Snapshot, versionID string) (*assessor.ScoreResult, error) {
			return &assessor.ScoreResult{
				SnapshotID: snapshot.ID,
				VersionID:  versionID,
				Evidence: []assessor.EvidenceItem{
					{Key: "a", RuleID: "r1", Severity: "info", Description: "d1"},
					{Key: "b", RuleID: "r2", Severity: "low", Description: "d2"},
					{Key: "c", RuleID: "r3", Severity: "medium", Description: "d3"},
					{Key: "d", RuleID: "r4", Severity: "medium", Description: "d4"},
					{Key: "e", RuleID: "r5", Severity: "high", Description: "d5"},
					{Key: "f", RuleID: "r6", Severity: "critical", Description: "d6"},
					{Key: "g", RuleID: "r7", Severity: "bogus-unknown", Description: "d7"},
				},
				Timestamp: time.Now(),
			}, nil
		},
	}
	a := newMokuForTest(t, cannedHappyPathWebClient(), multiSeverity)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if result.Summary == nil {
		t.Fatal("Summary is nil")
	}
	if result.Summary.Total != 7 {
		t.Errorf("Total = %d, want 7", result.Summary.Total)
	}
	if result.Summary.Info != 2 { // canonical "info" + unknown-falls-back-to-info
		t.Errorf("Info = %d, want 2 (includes unknown severity falling back to info)", result.Summary.Info)
	}
	if result.Summary.Low != 1 {
		t.Errorf("Low = %d, want 1", result.Summary.Low)
	}
	if result.Summary.Medium != 2 {
		t.Errorf("Medium = %d, want 2", result.Summary.Medium)
	}
	if result.Summary.High != 1 {
		t.Errorf("High = %d, want 1", result.Summary.High)
	}
	if result.Summary.Critical != 1 {
		t.Errorf("Critical = %d, want 1", result.Summary.Critical)
	}
	sum := result.Summary.Info + result.Summary.Low + result.Summary.Medium + result.Summary.High + result.Summary.Critical
	if sum != result.Summary.Total {
		t.Errorf("per-severity sum %d != Total %d", sum, result.Summary.Total)
	}
}

// TestMokuAnalyzer_ScanAndWait_StashesMokuScoresInRawData asserts the
// Moku-specific escape hatch: exposure and hardening scores go into
// RawData["moku.*"] rather than first-class fields on ScanResult.
func TestMokuAnalyzer_ScanAndWait_StashesMokuScoresInRawData(t *testing.T) {
	a := newMokuForTest(t, cannedHappyPathWebClient(), cannedHappyPathAssessor())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if result.RawData == nil {
		t.Fatal("RawData is nil; want Moku scores stashed under moku.* keys")
	}
	if _, ok := result.RawData["moku.exposure_score"]; !ok {
		t.Error(`RawData missing "moku.exposure_score"`)
	}
	if _, ok := result.RawData["moku.hardening_score"]; !ok {
		t.Error(`RawData missing "moku.hardening_score"`)
	}
}

// TestMokuAnalyzer_ScanAndWait_BackendAndJobIDConsistent asserts that the
// result's Backend field matches Name() and its JobID matches what
// SubmitScan returned.
func TestMokuAnalyzer_ScanAndWait_BackendAndJobIDConsistent(t *testing.T) {
	a := newMokuForTest(t, cannedHappyPathWebClient(), cannedHappyPathAssessor())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if result.Backend != analyzer.BackendMoku {
		t.Errorf("Backend = %q, want %q", result.Backend, analyzer.BackendMoku)
	}
	if result.Backend != a.Name() {
		t.Errorf("result.Backend %q does not match a.Name() %q", result.Backend, a.Name())
	}
	if result.JobID == "" {
		t.Error("JobID is empty")
	}
}

// TestMokuAnalyzer_SubmitScan_ReturnsQuicklyEvenForSlowPipeline asserts the
// async contract — SubmitScan must return promptly regardless of how long
// the internal pipeline takes to complete.
func TestMokuAnalyzer_SubmitScan_ReturnsQuicklyEvenForSlowPipeline(t *testing.T) {
	slow := &fakeWebClient{
		getFunc: func(ctx context.Context, url string) (*webclient.Response, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
			return &webclient.Response{StatusCode: 200, Body: []byte("<html/>"), FetchedAt: time.Now()}, nil
		},
	}
	a := newMokuForTest(t, slow, cannedHappyPathAssessor())

	start := time.Now()
	jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("SubmitScan: %v", err)
	}
	if jobID == "" {
		t.Fatal("empty job ID")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("SubmitScan took %s, want <100ms even with a slow webclient", elapsed)
	}
}

// TestMokuAnalyzer_ScanAndWait_WebclientError_ReturnsFailedStatus asserts the
// pipeline's failure handling for the fetch phase.
func TestMokuAnalyzer_ScanAndWait_WebclientError_ReturnsFailedStatus(t *testing.T) {
	boom := &fakeWebClient{
		getFunc: func(ctx context.Context, url string) (*webclient.Response, error) {
			return nil, errors.New("network unreachable")
		},
	}
	a := newMokuForTest(t, boom, cannedHappyPathAssessor())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://unreachable.example/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait returned err=%v; want nil with Status=Failed", err)
	}
	if result.Status != analyzer.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusFailed)
	}
	if result.Error == "" {
		t.Error("Error is empty; want webclient error message")
	}
}

// TestMokuAnalyzer_ScanAndWait_AssessorError_ReturnsFailedStatus asserts the
// pipeline's failure handling for the scoring phase.
func TestMokuAnalyzer_ScanAndWait_AssessorError_ReturnsFailedStatus(t *testing.T) {
	busted := &fakeAssessor{
		scoreFunc: func(ctx context.Context, snapshot *models.Snapshot, versionID string) (*assessor.ScoreResult, error) {
			return nil, errors.New("rule engine crashed")
		},
	}
	a := newMokuForTest(t, cannedHappyPathWebClient(), busted)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait returned err=%v; want nil with Status=Failed", err)
	}
	if result.Status != analyzer.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, analyzer.StatusFailed)
	}
	if result.Error == "" {
		t.Error("Error is empty; want assessor error message")
	}
}

// TestMokuAnalyzer_GetScan_SnapshotContainsFetchedBody asserts that the
// Moku backend feeds the webclient body into the assessor via a
// tracker/models.Snapshot. Captures the snapshot the assessor sees and
// verifies the body round-trips.
func TestMokuAnalyzer_GetScan_SnapshotContainsFetchedBody(t *testing.T) {
	const wantBody = `<html><head><title>t</title></head><body>x</body></html>`

	wc := &fakeWebClient{
		getFunc: func(ctx context.Context, url string) (*webclient.Response, error) {
			return &webclient.Response{
				StatusCode: 200,
				Body:       []byte(wantBody),
				FetchedAt:  time.Now(),
			}, nil
		},
	}

	var capturedBody []byte
	captureAs := &fakeAssessor{
		scoreFunc: func(ctx context.Context, snapshot *models.Snapshot, versionID string) (*assessor.ScoreResult, error) {
			capturedBody = snapshot.Body
			return &assessor.ScoreResult{SnapshotID: snapshot.ID, VersionID: versionID, Timestamp: time.Now()}, nil
		},
	}

	a := newMokuForTest(t, wc, captureAs)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{})
	if err != nil {
		t.Fatalf("ScanAndWait: %v", err)
	}
	if string(capturedBody) != wantBody {
		t.Errorf("assessor saw body %q, want %q", string(capturedBody), wantBody)
	}
}
