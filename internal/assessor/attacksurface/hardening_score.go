package attacksurface

import "strings"

// SecurityHeaderWeights maps security header names to their hardening score contribution.
// CSP is handled separately via ComputeCSPHardeningScore and has weight 0 here.
var SecurityHeaderWeights = map[string]float64{
	"strict-transport-security": 0.15,
	"x-frame-options":           0.10,
	"x-content-type-options":    0.05,
	"referrer-policy":           0.05,
	"permissions-policy":        0.05,
}

// ComputeHardeningScore computes a [0..1] hardening score from HTTP response headers.
// It checks for security header presence and parses CSP for quality scoring.
func ComputeHardeningScore(headers map[string][]string) float64 {
	if len(headers) == 0 {
		return 0.0
	}

	normalized := normalizeHeaderKeys(headers)

	var score float64

	for header, weight := range SecurityHeaderWeights {
		if _, ok := normalized[header]; ok {
			score += weight
		}
	}

	score += cspScore(normalized)

	return score
}

func cspScore(normalized map[string][]string) float64 {
	if vals, ok := normalized["content-security-policy"]; ok && len(vals) > 0 {
		csp := ParseCSP(strings.Join(vals, "; "), false)
		return ComputeCSPHardeningScore(csp)
	}
	if vals, ok := normalized["content-security-policy-report-only"]; ok && len(vals) > 0 {
		csp := ParseCSP(strings.Join(vals, "; "), true)
		return ComputeCSPHardeningScore(csp)
	}
	return 0.0
}

func normalizeHeaderKeys(headers map[string][]string) map[string][]string {
	normalized := make(map[string][]string, len(headers))
	for k, v := range headers {
		normalized[strings.ToLower(k)] = v
	}
	return normalized
}
