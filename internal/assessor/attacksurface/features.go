package attacksurface

import (
	"strings"
)

// ComputeFeatures extracts a fixed set of numeric / boolean features from an AttackSurface.
// The map is feature_name -> value (0.0 means "absent"/"false").
func ComputeFeatures(as *AttackSurface) map[string]float64 {
	f := map[string]float64{}

	if as == nil {
		return f
	}

	// -----------------------------------------------------------------------
	// 1) HTTP status / content-type
	// -----------------------------------------------------------------------

	if as.StatusCode >= 200 && as.StatusCode < 300 {
		f["status_2xx"] = 1
	} else if as.StatusCode >= 300 && as.StatusCode < 400 {
		f["status_3xx"] = 1
	} else if as.StatusCode >= 400 && as.StatusCode < 500 {
		f["status_4xx"] = 1
	} else if as.StatusCode >= 500 {
		f["status_5xx"] = 1
	}

	ct := strings.ToLower(as.ContentType)
	if strings.Contains(ct, "text/html") {
		f["is_html"] = 1
	}
	if strings.Contains(ct, "application/json") {
		f["is_json"] = 1
	}

	// -----------------------------------------------------------------------
	// 2) Security headers
	// -----------------------------------------------------------------------

	h := as.Headers // map[string][]string with lower-cased keys

	// Content-Security-Policy
	if cspVals, ok := h["content-security-policy"]; !ok || len(cspVals) == 0 {
		f["csp_missing"] = 1
	} else {
		joined := strings.ToLower(strings.Join(cspVals, ";"))
		if strings.Contains(joined, "unsafe-inline") {
			f["csp_unsafe_inline"] = 1
		}
		if strings.Contains(joined, "unsafe-eval") {
			f["csp_unsafe_eval"] = 1
		}
	}

	// X-Frame-Options
	if _, ok := h["x-frame-options"]; !ok {
		f["xfo_missing"] = 1
	}

	// X-Content-Type-Options
	if _, ok := h["x-content-type-options"]; !ok {
		f["xcto_missing"] = 1
	}

	// Strict-Transport-Security
	if _, ok := h["strict-transport-security"]; !ok {
		f["hsts_missing"] = 1
	}

	// Referrer-Policy
	if _, ok := h["referrer-policy"]; !ok {
		f["referrer_policy_missing"] = 1
	}

	// X-XSS-Protection (legacy, but might indicate hardening)
	if _, ok := h["x-xss-protection"]; ok {
		f["xxp_present"] = 1
	}

	// -----------------------------------------------------------------------
	// 3) Cookies
	// -----------------------------------------------------------------------

	var totalCookies, noHttpOnly, noSecure float64
	var hasSessionCookieNoHttpOnly float64

	for _, c := range as.Cookies {
		totalCookies++

		if !c.HttpOnly {
			noHttpOnly++
		}
		if !c.Secure {
			noSecure++
		}

		lname := strings.ToLower(c.Name)
		if strings.Contains(lname, "session") && !c.HttpOnly {
			hasSessionCookieNoHttpOnly = 1
		}
	}

	if totalCookies > 0 {
		f["num_cookies"] = totalCookies
	}
	if noHttpOnly > 0 {
		f["num_cookies_missing_httponly"] = noHttpOnly
	}
	if noSecure > 0 {
		f["num_cookies_missing_secure"] = noSecure
	}
	if hasSessionCookieNoHttpOnly > 0 {
		f["has_session_cookie_no_httponly"] = 1
	}

	// -----------------------------------------------------------------------
	// 4) Forms & inputs
	// -----------------------------------------------------------------------

	var (
		numForms, numInputs         float64
		numPwInputs, numFileInputs  float64
		numHiddenInputs             float64
		hasAdminForm, hasAuthForm   float64
		hasUploadForm, hasCSRFInput float64
	)

	for _, form := range as.Forms {
		numForms++

		// TODO: Improve detection of admin/auth/upload forms with larger keyword sets or ML?
		action := strings.ToLower(form.Action)
		if strings.Contains(action, "/admin") || strings.Contains(action, "admin") {
			hasAdminForm = 1
		}
		if strings.Contains(action, "login") || strings.Contains(action, "signin") || strings.Contains(action, "auth") {
			hasAuthForm = 1
		}
		if strings.Contains(action, "upload") || strings.Contains(action, "/upload") || strings.Contains(action, "file") {
			hasUploadForm = 1
		}

		for _, in := range form.Inputs {
			numInputs++

			t := strings.ToLower(in.Type)
			switch t {
			case "password":
				numPwInputs++
			case "file":
				numFileInputs++
			case "hidden":
				numHiddenInputs++
			}

			lname := strings.ToLower(in.Name)
			if strings.Contains(lname, "csrf") || strings.Contains(lname, "token") {
				hasCSRFInput = 1
			}
		}
	}

	if numForms > 0 {
		f["num_forms"] = numForms
	}
	if numInputs > 0 {
		f["num_inputs"] = numInputs
	}
	if numPwInputs > 0 {
		f["num_password_inputs"] = numPwInputs
		f["has_password_input"] = 1
	}
	if numFileInputs > 0 {
		f["num_file_inputs"] = numFileInputs
		f["has_file_upload"] = 1
	}
	if numHiddenInputs > 0 {
		f["num_hidden_inputs"] = numHiddenInputs
	}

	if hasAdminForm > 0 {
		f["has_admin_form"] = 1
	}
	if hasAuthForm > 0 {
		f["has_auth_form"] = 1
	}
	if hasUploadForm > 0 {
		f["has_upload_form"] = 1
	}
	if hasCSRFInput > 0 {
		f["has_csrf_input"] = 1
	}

	// -----------------------------------------------------------------------
	// 5) Params (GET + POST)
	// -----------------------------------------------------------------------

	combinedParams := make([]Param, 0, len(as.GetParams)+len(as.PostParams))
	combinedParams = append(combinedParams, as.GetParams...)
	combinedParams = append(combinedParams, as.PostParams...)

	var (
		numParams, numSuspicious float64
		hasAdminParam            float64
		hasUploadParam           float64
		hasDebugParam            float64
		hasIDParam               float64
	)

	for _, p := range combinedParams {
		if p.Name == "" {
			continue
		}
		numParams++

		lname := strings.ToLower(p.Name)

		// TODO: Expand suspicious param keyword list
		if strings.Contains(lname, "admin") {
			hasAdminParam = 1
			numSuspicious++
		}
		if strings.Contains(lname, "upload") || strings.Contains(lname, "file") {
			hasUploadParam = 1
			numSuspicious++
		}
		if strings.Contains(lname, "debug") || strings.Contains(lname, "test") || strings.Contains(lname, "dev") {
			hasDebugParam = 1
			numSuspicious++
		}
		if strings.Contains(lname, "id") {
			hasIDParam = 1
			numSuspicious++
		}
	}

	if numParams > 0 {
		f["num_params"] = numParams
	}
	if numSuspicious > 0 {
		f["num_suspicious_params"] = numSuspicious
	}
	if hasAdminParam > 0 {
		f["has_admin_param"] = 1
	}
	if hasUploadParam > 0 {
		f["has_upload_param"] = 1
	}
	if hasDebugParam > 0 {
		f["has_debug_param"] = 1
	}
	if hasIDParam > 0 {
		f["has_id_param"] = 1
	}

	// -----------------------------------------------------------------------
	// 6) Scripts
	// -----------------------------------------------------------------------

	var numScripts, numInlineScripts, numExternalScripts float64

	for _, s := range as.Scripts {
		numScripts++
		if s.Inline {
			numInlineScripts++
		} else if s.Src != "" {
			numExternalScripts++
		}
	}

	if numScripts > 0 {
		f["num_scripts"] = numScripts
	}
	if numInlineScripts > 0 {
		f["num_inline_scripts"] = numInlineScripts
	}
	if numExternalScripts > 0 {
		f["num_external_scripts"] = numExternalScripts
	}

	// -----------------------------------------------------------------------
	// 7) Error indicators & framework hints
	// -----------------------------------------------------------------------

	if len(as.ErrorIndicators) > 0 {
		f["has_error_indicators"] = 1
		f["num_error_indicators"] = float64(len(as.ErrorIndicators))
	}

	if len(as.FrameworkHints) > 0 {
		f["num_framework_hints"] = float64(len(as.FrameworkHints))
	}

	return f
}
