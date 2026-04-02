package attacksurface

import (
	"strings"
)

// CSPDirectives holds parsed Content-Security-Policy directive values.
type CSPDirectives struct {
	DefaultSrc []string
	ScriptSrc  []string
	ObjectSrc  []string
	BaseUri    []string
	FormAction []string
	FrameSrc   []string

	HasUnsafeInline bool
	HasUnsafeEval   bool
	ReportOnly      bool
}

// CSPHardeningWeights maps CSP directive names to their contribution weight
// toward the overall CSP hardening score.
var CSPHardeningWeights = map[string]float64{
	"script-src":  0.20,
	"object-src":  0.15,
	"base-uri":    0.10,
	"form-action": 0.10,
	"frame-src":   0.05,
	"default-src": 0.10,
}

// ParseCSP splits a CSP header value into structured directives.
func ParseCSP(header string, reportOnly bool) *CSPDirectives {
	csp := &CSPDirectives{
		ReportOnly: reportOnly,
	}

	if strings.TrimSpace(header) == "" {
		return csp
	}

	directives := strings.Split(header, ";")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive == "" {
			continue
		}

		parts := strings.Fields(directive)
		if len(parts) == 0 {
			continue
		}

		name := strings.ToLower(parts[0])
		values := parts[1:]

		switch name {
		case "default-src":
			csp.DefaultSrc = values
		case "script-src":
			csp.ScriptSrc = values
		case "object-src":
			csp.ObjectSrc = values
		case "base-uri":
			csp.BaseUri = values
		case "form-action":
			csp.FormAction = values
		case "frame-src":
			csp.FrameSrc = values
		}

		for _, v := range values {
			lower := strings.ToLower(v)
			if lower == "'unsafe-inline'" {
				csp.HasUnsafeInline = true
			}
			if lower == "'unsafe-eval'" {
				csp.HasUnsafeEval = true
			}
		}
	}

	return csp
}

// ComputeCSPHardeningScore computes a [0..1] hardening score from parsed CSP directives.
// Report-only policies return 0. Directives containing 'none' or 'self' get full credit;
// unsafe-inline/eval gets partial credit.
func ComputeCSPHardeningScore(csp *CSPDirectives) float64 {
	if csp == nil {
		return 0.0
	}
	if csp.ReportOnly {
		return 0.0
	}

	var score float64

	directiveValues := map[string][]string{
		"default-src": csp.DefaultSrc,
		"script-src":  csp.ScriptSrc,
		"object-src":  csp.ObjectSrc,
		"base-uri":    csp.BaseUri,
		"form-action": csp.FormAction,
		"frame-src":   csp.FrameSrc,
	}

	for directive, values := range directiveValues {
		weight, ok := CSPHardeningWeights[directive]
		if !ok || len(values) == 0 {
			continue
		}

		credit := directiveCredit(values)
		score += weight * credit
	}

	return score
}

// directiveCredit returns [0..1] credit for a directive's value list.
// 'none' = full credit, 'self' = full credit, unsafe-inline/eval = half credit,
// wildcard = no credit.
func directiveCredit(values []string) float64 {
	if containsNone(values) {
		return 1.0
	}
	if containsWildcard(values) {
		return 0.0
	}

	credit := 1.0
	for _, v := range values {
		lower := strings.ToLower(v)
		if lower == "'unsafe-inline'" {
			credit *= 0.5
		}
		if lower == "'unsafe-eval'" {
			credit *= 0.5
		}
	}
	return credit
}

func containsNone(values []string) bool {
	for _, v := range values {
		if strings.ToLower(v) == "'none'" {
			return true
		}
	}
	return false
}

func containsSelf(values []string) bool {
	for _, v := range values {
		if strings.ToLower(v) == "'self'" {
			return true
		}
	}
	return false
}

func containsWildcard(values []string) bool {
	for _, v := range values {
		if v == "*" {
			return true
		}
	}
	return false
}
