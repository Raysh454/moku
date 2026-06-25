package grading_test

import (
	"net/http"
	"testing"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/webclient"
)

func TestBodyMarkerSignal_DetectsMarkerCaseInsensitively(t *testing.T) {
	t.Parallel()

	sig := grading.NewBodyMarkerSignal("cloudflare-challenge", "Just a moment")
	resp := &webclient.Response{
		StatusCode: 200,
		Body:       []byte("<title>JUST A MOMENT...</title>"),
	}

	res := sig.Evaluate(resp)

	if res.Name != "cloudflare-challenge" {
		t.Errorf("expected signal name carried through, got %q", res.Name)
	}
	if !res.Detected {
		t.Fatal("expected marker to be detected case-insensitively")
	}
	if res.Evidence == "" {
		t.Error("expected non-empty evidence when detected")
	}
}

func TestBodyMarkerSignal_CleanBody_NotDetected(t *testing.T) {
	t.Parallel()

	sig := grading.NewBodyMarkerSignal("cloudflare-challenge", "Just a moment")
	resp := &webclient.Response{StatusCode: 200, Body: []byte("<h1>Welcome</h1>")}

	if sig.Evaluate(resp).Detected {
		t.Error("did not expect detection on a clean body")
	}
}

func TestBodyMarkerSignal_NilResponse_NotDetected(t *testing.T) {
	t.Parallel()

	sig := grading.NewBodyMarkerSignal("x", "y")

	if sig.Evaluate(nil).Detected {
		t.Error("nil response must never report a detection")
	}
}

func TestStatusSignal_DetectsConfiguredCode(t *testing.T) {
	t.Parallel()

	sig := grading.NewStatusSignal("blocked-status", 403, 429, 503)

	if !sig.Evaluate(&webclient.Response{StatusCode: 403}).Detected {
		t.Error("expected 403 to be detected")
	}
	if sig.Evaluate(&webclient.Response{StatusCode: 200}).Detected {
		t.Error("did not expect 200 to be detected")
	}
}

func TestHeaderMarkerSignal_DetectsHeaderValue(t *testing.T) {
	t.Parallel()

	sig := grading.NewHeaderMarkerSignal("cf-mitigated", "Cf-Mitigated", "challenge")
	hdr := http.Header{}
	hdr.Set("Cf-Mitigated", "challenge")
	resp := &webclient.Response{StatusCode: 403, Headers: hdr}

	if !sig.Evaluate(resp).Detected {
		t.Error("expected cf-mitigated header marker to be detected")
	}

	clean := &webclient.Response{StatusCode: 200, Headers: http.Header{}}
	if sig.Evaluate(clean).Detected {
		t.Error("did not expect detection when header is absent")
	}
}
