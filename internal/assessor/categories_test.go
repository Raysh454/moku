package assessor_test

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

func TestCategoryForFeature_AllAttackSurfaceFeatures_ReturnExpectedCategory(t *testing.T) {
	t.Parallel()

	headerFeatures := []string{
		"csp_missing", "csp_unsafe_inline", "csp_unsafe_eval",
		"xfo_missing", "xcto_missing", "hsts_missing",
		"referrer_policy_missing", "xxp_present",
	}
	cookieFeatures := []string{
		"num_cookies", "num_cookies_missing_httponly",
		"num_cookies_missing_secure", "has_session_cookie_no_httponly",
	}
	formFeatures := []string{
		"num_forms", "num_inputs", "num_password_inputs",
		"num_file_inputs", "num_hidden_inputs",
		"has_password_input", "has_file_upload", "has_csrf_input",
		"has_admin_form", "has_auth_form", "has_upload_form",
	}
	paramFeatures := []string{
		"num_params", "num_suspicious_params",
		"has_admin_param", "has_upload_param",
		"has_debug_param", "has_id_param",
	}
	scriptFeatures := []string{
		"num_scripts", "num_inline_scripts", "num_external_scripts",
	}
	infoLeakFeatures := []string{
		"status_2xx", "status_3xx", "status_4xx", "status_5xx",
		"is_html", "is_json",
		"has_error_indicators", "num_error_indicators", "num_framework_hints",
	}

	tests := []struct {
		features []string
		expected assessor.Category
	}{
		{headerFeatures, assessor.CategoryHeaders},
		{cookieFeatures, assessor.CategoryCookies},
		{formFeatures, assessor.CategoryForms},
		{paramFeatures, assessor.CategoryParams},
		{scriptFeatures, assessor.CategoryScripts},
		{infoLeakFeatures, assessor.CategoryInfoLeak},
	}

	for _, tt := range tests {
		for _, feat := range tt.features {
			got := assessor.CategoryForFeature(feat)
			if got != tt.expected {
				t.Errorf("CategoryForFeature(%q) = %q, want %q", feat, got, tt.expected)
			}
		}
	}
}

func TestCategoryForFeature_DOMRules_ReturnDOMHygiene(t *testing.T) {
	t.Parallel()

	domRuleIDs := []string{
		"dom:inline-event-handler", "dom:javascript-href", "dom:base-tag",
		"dom:form-http-action", "dom:iframe-src", "dom:meta-refresh",
		"dom:hardcoded-secret", "dom:debug-artifact", "dom:dev-comment",
	}

	for _, id := range domRuleIDs {
		got := assessor.CategoryForFeature(id)
		if got != assessor.CategoryDOMHygiene {
			t.Errorf("CategoryForFeature(%q) = %q, want %q", id, got, assessor.CategoryDOMHygiene)
		}
	}
}

func TestCategoryForFeature_UnknownFeature_ReturnInfoLeak(t *testing.T) {
	t.Parallel()

	got := assessor.CategoryForFeature("some_unknown_feature")
	if got != assessor.CategoryInfoLeak {
		t.Errorf("CategoryForFeature(unknown) = %q, want %q", got, assessor.CategoryInfoLeak)
	}
}

func TestCategoryForFeature_CoversAllFeatureWeights(t *testing.T) {
	t.Parallel()

	for name := range attacksurface.FeatureWeights {
		cat := assessor.CategoryForFeature(name)
		if cat == "" {
			t.Errorf("CategoryForFeature(%q) returned empty category", name)
		}
	}
}

func TestAllCategories_ReturnsSevenCategories(t *testing.T) {
	t.Parallel()

	cats := assessor.AllCategories()
	if len(cats) != 7 {
		t.Errorf("AllCategories() returned %d categories, want 7", len(cats))
	}

	expected := map[assessor.Category]bool{
		assessor.CategoryHeaders:    true,
		assessor.CategoryCookies:    true,
		assessor.CategoryForms:      true,
		assessor.CategoryParams:     true,
		assessor.CategoryScripts:    true,
		assessor.CategoryDOMHygiene: true,
		assessor.CategoryInfoLeak:   true,
	}
	for _, c := range cats {
		if !expected[c] {
			t.Errorf("unexpected category %q in AllCategories()", c)
		}
	}
}
