package attacksurface

// featureWeights maps feature names to their contribution weights.
// These are heuristic and can be tuned over time.
var FeatureWeights = map[string]float64{
	// Status / content-type — usually very low weight
	"status_2xx": 0.0,
	"status_3xx": 0.0,
	"status_4xx": 0.0,
	"status_5xx": 0.0,
	"is_html":    0.0,
	"is_json":    0.0,

	// Security headers
	"csp_missing":             0.5,
	"csp_unsafe_inline":       0.2,
	"csp_unsafe_eval":         0.2,
	"xfo_missing":             0.2,
	"xcto_missing":            0.1,
	"hsts_missing":            0.2,
	"referrer_policy_missing": 0.1,
	"xxp_present":             0.0, // purely informational for now

	// Cookies
	"num_cookies":                    0.0,
	"num_cookies_missing_httponly":   0.05, // per cookie
	"num_cookies_missing_secure":     0.05, // per cookie
	"has_session_cookie_no_httponly": 0.2,

	// Forms & inputs
	"num_forms":           0.0,
	"num_inputs":          0.0,
	"num_password_inputs": 0.1,
	"num_file_inputs":     0.1,
	"num_hidden_inputs":   0.0,

	"has_password_input": 0.1,
	"has_file_upload":    0.4,
	"has_csrf_input":     -0.05, // negative weight -> slightly reduces risk

	"has_admin_form":  0.2,
	"has_auth_form":   0.1,
	"has_upload_form": 0.2,

	// Params
	"num_params":            0.0,
	"num_suspicious_params": 0.1,
	"has_admin_param":       0.2,
	"has_upload_param":      0.2,
	"has_debug_param":       0.1,
	"has_id_param":          0.05,

	// Scripts
	"num_scripts":          0.0,
	"num_inline_scripts":   0.05,
	"num_external_scripts": 0.0,

	// Errors & frameworks
	"has_error_indicators": 0.2,
	"num_error_indicators": 0.1,
	"num_framework_hints":  0.0,
}

// severityForFeature returns a coarse severity label for a feature.
func SeverityForFeature(name string) string {
	switch name {
	// High-impact issues
	case "csp_missing",
		"has_file_upload",
		"has_admin_form",
		"has_admin_param",
		"has_session_cookie_no_httponly":
		return "high"

	// Medium-impact
	case "csp_unsafe_inline",
		"csp_unsafe_eval",
		"hsts_missing",
		"xfo_missing",
		"has_upload_form",
		"has_upload_param",
		"has_error_indicators",
		"num_suspicious_params":
		return "medium"

	// Low-impact / informational
	default:
		return "low"
	}
}

// describeFeature returns a short human-readable explanation of a feature.
func DescribeFeature(name string) string {
	switch name {

	// Status / content-type
	case "status_2xx":
		return "Page returned a 2xx success status code"
	case "status_3xx":
		return "Page returned a 3xx redirect status code"
	case "status_4xx":
		return "Page returned a 4xx client error status code"
	case "status_5xx":
		return "Page returned a 5xx server error status code"
	case "is_html":
		return "Response content appears to be HTML"
	case "is_json":
		return "Response content appears to be JSON"

	// Security headers
	case "csp_missing":
		return "Content-Security-Policy header is missing"
	case "csp_unsafe_inline":
		return "CSP allows 'unsafe-inline', increasing XSS risk"
	case "csp_unsafe_eval":
		return "CSP allows 'unsafe-eval', increasing XSS risk"
	case "xfo_missing":
		return "X-Frame-Options header is missing (clickjacking risk)"
	case "xcto_missing":
		return "X-Content-Type-Options header is missing (MIME sniffing risk)"
	case "hsts_missing":
		return "Strict-Transport-Security header is missing (HTTPS downgrade risk)"
	case "referrer_policy_missing":
		return "Referrer-Policy header is missing"
	case "xxp_present":
		return "X-XSS-Protection header is present (legacy browser XSS filter)"

	// Cookies
	case "num_cookies":
		return "Number of cookies set by the response"
	case "num_cookies_missing_httponly":
		return "Cookies missing the HttpOnly flag"
	case "num_cookies_missing_secure":
		return "Cookies missing the Secure flag"
	case "has_session_cookie_no_httponly":
		return "Session-like cookie missing HttpOnly flag"

	// Forms & inputs
	case "num_forms":
		return "Number of HTML forms on the page"
	case "num_inputs":
		return "Number of input elements in forms"
	case "num_password_inputs":
		return "Number of password inputs in forms"
	case "num_file_inputs":
		return "Number of file upload inputs in forms"
	case "num_hidden_inputs":
		return "Number of hidden inputs in forms"
	case "has_password_input":
		return "Page contains password input fields"
	case "has_file_upload":
		return "Page exposes file upload functionality"
	case "has_csrf_input":
		return "Page includes inputs that look like CSRF tokens"
	case "has_admin_form":
		return "Page contains forms targeting admin-like paths"
	case "has_auth_form":
		return "Page contains login/authentication forms"
	case "has_upload_form":
		return "Page contains forms targeting upload endpoints"

	// Params
	case "num_params":
		return "Number of query or body parameters"
	case "num_suspicious_params":
		return "Parameters with suspicious names (admin/upload/debug/id)"
	case "has_admin_param":
		return "Parameters suggesting administrative functionality"
	case "has_upload_param":
		return "Parameters suggesting upload or file operations"
	case "has_debug_param":
		return "Parameters suggesting debug or test functionality"
	case "has_id_param":
		return "Parameters containing 'id' in their name"

	// Scripts
	case "num_scripts":
		return "Number of script tags on the page"
	case "num_inline_scripts":
		return "Number of inline script tags on the page"
	case "num_external_scripts":
		return "Number of external script tags on the page"

	// Errors & frameworks
	case "has_error_indicators":
		return "Page contains error indicators or stack trace-like content"
	case "num_error_indicators":
		return "Number of detected error indicators in the page"
	case "num_framework_hints":
		return "Number of detected framework hints (e.g., technology banners)"

	default:
		// Fallback: just echo the feature name
		return name
	}
}
