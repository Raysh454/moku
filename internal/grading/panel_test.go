package grading_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/webclient"
)

func TestDefaultPanel_IsNonEmptyHTTPSProbes(t *testing.T) {
	t.Parallel()

	panel := grading.DefaultPanel()
	if len(panel) == 0 {
		t.Fatal("expected a non-empty default panel")
	}
	seen := make(map[string]bool)
	for _, p := range panel {
		if p.Name == "" {
			t.Errorf("probe %q has empty name", p.URL)
		}
		if !strings.HasPrefix(p.URL, "https://") {
			t.Errorf("probe %q is not https", p.Name)
		}
		if seen[p.Name] {
			t.Errorf("duplicate probe name %q", p.Name)
		}
		seen[p.Name] = true
	}
}

func TestDefaultClassifier_JustAMoment_IsChallenged(t *testing.T) {
	t.Parallel()

	c := grading.DefaultClassifier()
	resp := &webclient.Response{StatusCode: 200, Body: []byte("<title>Just a moment...</title>")}

	if got := c.Classify(resp).Outcome; got != grading.OutcomeChallenged {
		t.Errorf("expected Challenged, got %q", got)
	}
}

func TestDefaultClassifier_Forbidden_IsBlocked(t *testing.T) {
	t.Parallel()

	c := grading.DefaultClassifier()
	resp := &webclient.Response{StatusCode: 403, Body: []byte("denied"), Headers: http.Header{}}

	if got := c.Classify(resp).Outcome; got != grading.OutcomeBlocked {
		t.Errorf("expected Blocked, got %q", got)
	}
}

func TestDefaultClassifier_CleanContent_IsOK(t *testing.T) {
	t.Parallel()

	c := grading.DefaultClassifier()
	resp := &webclient.Response{StatusCode: 200, Body: []byte("<html><body>real content</body></html>"), Headers: http.Header{}}

	if got := c.Classify(resp).Outcome; got != grading.OutcomeOK {
		t.Errorf("expected OK, got %q", got)
	}
}

func TestDefaultClassifier_CfMitigatedHeader_IsChallenged(t *testing.T) {
	t.Parallel()

	c := grading.DefaultClassifier()
	hdr := http.Header{}
	hdr.Set("Cf-Mitigated", "challenge")
	resp := &webclient.Response{StatusCode: 200, Body: []byte("ok"), Headers: hdr}

	if got := c.Classify(resp).Outcome; got != grading.OutcomeChallenged {
		t.Errorf("expected Challenged from cf-mitigated header, got %q", got)
	}
}
