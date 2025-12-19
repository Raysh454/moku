package attacksurface

import "testing"

func TestComputeFeatures_NilAttackSurface(t *testing.T) {
	got := ComputeFeatures(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty feature map for nil AttackSurface, got %v", got)
	}
}

func TestComputeFeatures_BasicExtraction(t *testing.T) {
	as := &AttackSurface{
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string][]string{
			"content-security-policy": {"default-src 'self'; script-src 'unsafe-inline' 'unsafe-eval'"},
			"x-xss-protection":        {"1; mode=block"},
		},
		Cookies: []CookieInfo{
			{Name: "sessionid", Secure: false, HttpOnly: false},
			{Name: "analytics", Secure: true, HttpOnly: true},
		},
		Forms: []Form{
			{
				Action: "/admin/login",
				Method: "POST",
				Inputs: []FormInput{
					{Name: "username", Type: "text"},
					{Name: "password", Type: "password"},
					{Name: "csrf_token", Type: "hidden"},
				},
			},
		},
		GetParams: []Param{
			{Name: "admin", Origin: "query"},
			{Name: "id", Origin: "query"},
		},
		PostParams: []Param{
			{Name: "upload_file", Origin: "body"},
		},
		Scripts: []ScriptInfo{
			{Inline: true},
			{Src: "https://cdn.example.com/app.js", Inline: false},
		},
		ErrorIndicators: []string{"stack trace"},
		FrameworkHints:  []string{"django"},
	}

	got := ComputeFeatures(as)

	// Status and content-type
	if got["status_2xx"] != 1 {
		t.Errorf("expected status_2xx == 1, got %v", got["status_2xx"])
	}
	if got["is_html"] != 1 {
		t.Errorf("expected is_html == 1, got %v", got["is_html"])
	}

	// Security headers
	if got["csp_missing"] != 0 {
		t.Errorf("expected csp_missing == 0, got %v", got["csp_missing"])
	}
	if got["csp_unsafe_inline"] != 1 {
		t.Errorf("expected csp_unsafe_inline == 1, got %v", got["csp_unsafe_inline"])
	}
	if got["csp_unsafe_eval"] != 1 {
		t.Errorf("expected csp_unsafe_eval == 1, got %v", got["csp_unsafe_eval"])
	}
	if got["xfo_missing"] != 1 {
		t.Errorf("expected xfo_missing == 1 (no X-Frame-Options header), got %v", got["xfo_missing"])
	}
	if got["xcto_missing"] != 1 {
		t.Errorf("expected xcto_missing == 1 (no X-Content-Type-Options header), got %v", got["xcto_missing"])
	}
	if got["hsts_missing"] != 1 {
		t.Errorf("expected hsts_missing == 1 (no HSTS header), got %v", got["hsts_missing"])
	}
	if got["referrer_policy_missing"] != 1 {
		t.Errorf("expected referrer_policy_missing == 1, got %v", got["referrer_policy_missing"])
	}
	if got["xxp_present"] != 1 {
		t.Errorf("expected xxp_present == 1 when X-XSS-Protection present, got %v", got["xxp_present"])
	}

	// Cookies
	if got["num_cookies"] != 2 {
		t.Errorf("expected num_cookies == 2, got %v", got["num_cookies"])
	}
	if got["num_cookies_missing_httponly"] != 1 {
		t.Errorf("expected one cookie missing HttpOnly, got %v", got["num_cookies_missing_httponly"])
	}
	if got["num_cookies_missing_secure"] != 1 {
		t.Errorf("expected one cookie missing Secure, got %v", got["num_cookies_missing_secure"])
	}
	if got["has_session_cookie_no_httponly"] != 1 {
		t.Errorf("expected has_session_cookie_no_httponly == 1, got %v", got["has_session_cookie_no_httponly"])
	}

	// Forms & inputs
	if got["num_forms"] != 1 {
		t.Errorf("expected num_forms == 1, got %v", got["num_forms"])
	}
	if got["num_inputs"] != 3 {
		t.Errorf("expected num_inputs == 3, got %v", got["num_inputs"])
	}
	if got["num_password_inputs"] != 1 {
		t.Errorf("expected num_password_inputs == 1, got %v", got["num_password_inputs"])
	}
	if got["num_hidden_inputs"] != 1 {
		t.Errorf("expected num_hidden_inputs == 1, got %v", got["num_hidden_inputs"])
	}
	if got["has_password_input"] != 1 {
		t.Errorf("expected has_password_input == 1, got %v", got["has_password_input"])
	}
	if got["has_admin_form"] != 1 {
		t.Errorf("expected has_admin_form == 1, got %v", got["has_admin_form"])
	}
	if got["has_auth_form"] != 1 {
		t.Errorf("expected has_auth_form == 1, got %v", got["has_auth_form"])
	}
	if got["has_csrf_input"] != 1 {
		t.Errorf("expected has_csrf_input == 1, got %v", got["has_csrf_input"])
	}

	// Params
	if got["num_params"] != 3 {
		t.Errorf("expected num_params == 3, got %v", got["num_params"])
	}
	if got["num_suspicious_params"] != 3 {
		t.Errorf("expected num_suspicious_params == 3, got %v", got["num_suspicious_params"])
	}
	if got["has_admin_param"] != 1 {
		t.Errorf("expected has_admin_param == 1, got %v", got["has_admin_param"])
	}
	if got["has_upload_param"] != 1 {
		t.Errorf("expected has_upload_param == 1, got %v", got["has_upload_param"])
	}
	if got["has_id_param"] != 1 {
		t.Errorf("expected has_id_param == 1, got %v", got["has_id_param"])
	}

	// Scripts
	if got["num_scripts"] != 2 {
		t.Errorf("expected num_scripts == 2, got %v", got["num_scripts"])
	}
	if got["num_inline_scripts"] != 1 {
		t.Errorf("expected num_inline_scripts == 1, got %v", got["num_inline_scripts"])
	}
	if got["num_external_scripts"] != 1 {
		t.Errorf("expected num_external_scripts == 1, got %v", got["num_external_scripts"])
	}

	// Errors & frameworks
	if got["has_error_indicators"] != 1 {
		t.Errorf("expected has_error_indicators == 1, got %v", got["has_error_indicators"])
	}
	if got["num_error_indicators"] != 1 {
		t.Errorf("expected num_error_indicators == 1, got %v", got["num_error_indicators"])
	}
	if got["num_framework_hints"] != 1 {
		t.Errorf("expected num_framework_hints == 1, got %v", got["num_framework_hints"])
	}
}
