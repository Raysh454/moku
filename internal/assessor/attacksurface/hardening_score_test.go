package attacksurface

import "testing"

func TestComputeHardeningScore_NoHeaders(t *testing.T) {
	score := ComputeHardeningScore(nil)
	if score != 0.0 {
		t.Errorf("expected 0.0 for nil headers, got %v", score)
	}
}

func TestComputeHardeningScore_EmptyHeaders(t *testing.T) {
	score := ComputeHardeningScore(map[string][]string{})
	if score != 0.0 {
		t.Errorf("expected 0.0 for empty headers, got %v", score)
	}
}

func TestComputeHardeningScore_AllSecurityHeaders_StrongCSP(t *testing.T) {
	headers := map[string][]string{
		"strict-transport-security": {"max-age=31536000; includeSubDomains"},
		"x-frame-options":           {"DENY"},
		"x-content-type-options":    {"nosniff"},
		"referrer-policy":           {"no-referrer"},
		"permissions-policy":        {"geolocation=()"},
		"content-security-policy":   {"default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"},
	}
	score := ComputeHardeningScore(headers)

	if score < 0.8 {
		t.Errorf("expected high score for full security headers + strong CSP, got %v", score)
	}
}

func TestComputeHardeningScore_HeadersWithoutCSP(t *testing.T) {
	headers := map[string][]string{
		"strict-transport-security": {"max-age=31536000"},
		"x-frame-options":           {"DENY"},
		"x-content-type-options":    {"nosniff"},
		"referrer-policy":           {"no-referrer"},
		"permissions-policy":        {"geolocation=()"},
	}
	score := ComputeHardeningScore(headers)

	fullHeaders := map[string][]string{
		"strict-transport-security": {"max-age=31536000"},
		"x-frame-options":           {"DENY"},
		"x-content-type-options":    {"nosniff"},
		"referrer-policy":           {"no-referrer"},
		"permissions-policy":        {"geolocation=()"},
		"content-security-policy":   {"default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"},
	}
	fullScore := ComputeHardeningScore(fullHeaders)

	if score >= fullScore {
		t.Errorf("expected score without CSP (%v) < score with CSP (%v)", score, fullScore)
	}
}

func TestComputeHardeningScore_CSPReportOnly(t *testing.T) {
	headers := map[string][]string{
		"content-security-policy-report-only": {"default-src 'self'; script-src 'self'"},
	}
	score := ComputeHardeningScore(headers)
	if score != 0.0 {
		t.Errorf("expected 0.0 for report-only CSP alone, got %v", score)
	}
}

func TestComputeHardeningScore_CaseInsensitiveHeaders(t *testing.T) {
	lower := ComputeHardeningScore(map[string][]string{
		"strict-transport-security": {"max-age=31536000"},
	})
	upper := ComputeHardeningScore(map[string][]string{
		"Strict-Transport-Security": {"max-age=31536000"},
	})
	if lower != upper {
		t.Errorf("expected case-insensitive matching: lower=%v upper=%v", lower, upper)
	}
}
