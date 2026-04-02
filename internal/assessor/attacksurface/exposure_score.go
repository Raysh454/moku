package attacksurface

import (
	"math"
	"strings"
)

// ComputeExposureScore computes a [0..1] exposure score directly from the AttackSurface elements.
// It uses ElementScores for per-element scoring and optional SaturationConfig for count capping.
func ComputeExposureScore(as *AttackSurface, sat *SaturationConfig) float64 {
	if as == nil {
		return 0.0
	}

	var raw float64

	raw += scoreForms(as)
	raw += scoreInputs(as)
	raw += scoreCookies(as)
	raw += scoreScripts(as, sat)
	raw += scoreParams(as, sat)

	return raw
}

func scoreForms(as *AttackSurface) float64 {
	var total float64
	for _, form := range as.Forms {
		action := strings.ToLower(form.Action)
		switch {
		case containsAny(action, "admin", "/admin"):
			total += ElementScores["form_admin"]
		case containsAny(action, "login", "signin", "auth"):
			total += ElementScores["form_auth"]
		case containsAny(action, "upload", "/upload", "file"):
			total += ElementScores["form_upload"]
		default:
			total += ElementScores["form"]
		}
	}
	return total
}

func scoreInputs(as *AttackSurface) float64 {
	var total float64
	for _, form := range as.Forms {
		for _, in := range form.Inputs {
			t := strings.ToLower(in.Type)
			switch t {
			case "file":
				total += ElementScores["input_file"]
			case "password":
				total += ElementScores["input_password"]
			case "hidden":
				total += ElementScores["input_hidden"]
			default:
				total += ElementScores["input"]
			}
		}
	}
	return total
}

func scoreCookies(as *AttackSurface) float64 {
	var total float64
	for _, c := range as.Cookies {
		if strings.Contains(strings.ToLower(c.Name), "session") {
			total += ElementScores["cookie_session"]
		}
		if !c.HttpOnly {
			total += ElementScores["cookie_no_httponly"]
		}
		if !c.Secure {
			total += ElementScores["cookie_no_secure"]
		}
		total += ElementScores["cookie"]
	}
	return total
}

func scoreScripts(as *AttackSurface, sat *SaturationConfig) float64 {
	inlineCount := countInlineScripts(as)
	externalCount := float64(len(as.Scripts)) - inlineCount

	inlineCount = saturate(inlineCount, "script", sat)
	externalCount = saturate(externalCount, "script", sat)

	return inlineCount*ElementScores["script_inline"] + externalCount*ElementScores["script"]
}

func scoreParams(as *AttackSurface, sat *SaturationConfig) float64 {
	var total, suspicious float64
	allParams := append(as.GetParams, as.PostParams...)
	for _, p := range allParams {
		if p.Name == "" {
			continue
		}
		total++
		lname := strings.ToLower(p.Name)
		if containsAny(lname, "admin", "upload", "file", "debug", "test", "dev", "id") {
			suspicious++
		}
	}

	total = saturate(total, "param", sat)
	suspicious = math.Min(suspicious, total)

	return total*ElementScores["param"] + suspicious*ElementScores["param_suspicious"]
}

func countInlineScripts(as *AttackSurface) float64 {
	var count float64
	for _, s := range as.Scripts {
		if s.Inline {
			count++
		}
	}
	return count
}

// saturate caps count at the saturation limit for the given element type.
// Returns raw count if saturation is nil or disabled.
func saturate(count float64, elementType string, sat *SaturationConfig) float64 {
	if sat == nil || !sat.Enabled {
		return count
	}
	cap, ok := sat.Caps[elementType]
	if !ok {
		return count
	}
	return math.Min(count, cap)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
