package grading_test

import (
	"net/http"
	"testing"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/webclient"
)

func challengeRule() grading.ClassifyRule {
	return grading.ClassifyRule{
		Signal:  grading.NewBodyMarkerSignal("cloudflare-challenge", "just a moment"),
		Outcome: grading.OutcomeChallenged,
	}
}

func blockedRule() grading.ClassifyRule {
	return grading.ClassifyRule{
		Signal:  grading.NewStatusSignal("blocked-status", 403, 429, 503),
		Outcome: grading.OutcomeBlocked,
	}
}

func TestClassifier_NoRules_CleanResponse_IsOK(t *testing.T) {
	t.Parallel()

	c := grading.NewClassifier()
	res := c.Classify(&webclient.Response{StatusCode: 200, Body: []byte("hello")})

	if res.Outcome != grading.OutcomeOK {
		t.Errorf("expected OK, got %q", res.Outcome)
	}
	if len(res.Triggered) != 0 {
		t.Errorf("expected no triggered signals, got %d", len(res.Triggered))
	}
}

func TestClassifier_ChallengeMarker_IsChallenged(t *testing.T) {
	t.Parallel()

	c := grading.NewClassifier(challengeRule(), blockedRule())
	resp := &webclient.Response{StatusCode: 200, Body: []byte("<title>Just a moment...</title>")}

	res := c.Classify(resp)

	if res.Outcome != grading.OutcomeChallenged {
		t.Errorf("expected Challenged, got %q", res.Outcome)
	}
	if len(res.Triggered) != 1 {
		t.Fatalf("expected exactly one triggered signal, got %d", len(res.Triggered))
	}
}

func TestClassifier_BlockedStatus_IsBlocked(t *testing.T) {
	t.Parallel()

	c := grading.NewClassifier(challengeRule(), blockedRule())
	res := c.Classify(&webclient.Response{StatusCode: 403, Body: []byte("forbidden")})

	if res.Outcome != grading.OutcomeBlocked {
		t.Errorf("expected Blocked, got %q", res.Outcome)
	}
}

func TestClassifier_MostSevereOutcomeWins(t *testing.T) {
	t.Parallel()

	// Both a challenge marker AND a blocked status are present; Blocked is more
	// severe and must win, but both signals are reported as evidence.
	c := grading.NewClassifier(challengeRule(), blockedRule())
	resp := &webclient.Response{
		StatusCode: 503,
		Body:       []byte("<title>Just a moment...</title>"),
		Headers:    http.Header{},
	}

	res := c.Classify(resp)

	if res.Outcome != grading.OutcomeBlocked {
		t.Errorf("expected Blocked to win over Challenged, got %q", res.Outcome)
	}
	if len(res.Triggered) != 2 {
		t.Errorf("expected both signals reported as evidence, got %d", len(res.Triggered))
	}
}

func TestClassifier_NilResponse_IsError(t *testing.T) {
	t.Parallel()

	c := grading.NewClassifier(challengeRule(), blockedRule())

	if got := c.Classify(nil).Outcome; got != grading.OutcomeError {
		t.Errorf("expected Error for nil response, got %q", got)
	}
}
