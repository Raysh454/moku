package assessor

import "strings"

type Category string

const (
	CategoryHeaders    Category = "headers"
	CategoryCookies    Category = "cookies"
	CategoryForms      Category = "forms"
	CategoryParams     Category = "params"
	CategoryScripts    Category = "scripts"
	CategoryDOMHygiene Category = "dom_hygiene"
	CategoryInfoLeak   Category = "info_leak"
)

var featureCategoryMap = map[string]Category{
	// Headers
	"csp_missing":             CategoryHeaders,
	"csp_unsafe_inline":       CategoryHeaders,
	"csp_unsafe_eval":         CategoryHeaders,
	"xfo_missing":             CategoryHeaders,
	"xcto_missing":            CategoryHeaders,
	"hsts_missing":            CategoryHeaders,
	"referrer_policy_missing": CategoryHeaders,
	"xxp_present":             CategoryHeaders,

	// Cookies
	"num_cookies":                    CategoryCookies,
	"num_cookies_missing_httponly":   CategoryCookies,
	"num_cookies_missing_secure":     CategoryCookies,
	"has_session_cookie_no_httponly": CategoryCookies,

	// Forms & inputs
	"num_forms":           CategoryForms,
	"num_inputs":          CategoryForms,
	"num_password_inputs": CategoryForms,
	"num_file_inputs":     CategoryForms,
	"num_hidden_inputs":   CategoryForms,
	"has_password_input":  CategoryForms,
	"has_file_upload":     CategoryForms,
	"has_csrf_input":      CategoryForms,
	"has_admin_form":      CategoryForms,
	"has_auth_form":       CategoryForms,
	"has_upload_form":     CategoryForms,

	// Params
	"num_params":            CategoryParams,
	"num_suspicious_params": CategoryParams,
	"has_admin_param":       CategoryParams,
	"has_upload_param":      CategoryParams,
	"has_debug_param":       CategoryParams,
	"has_id_param":          CategoryParams,

	// Scripts
	"num_scripts":          CategoryScripts,
	"num_inline_scripts":   CategoryScripts,
	"num_external_scripts": CategoryScripts,

	// Info leak
	"status_2xx":           CategoryInfoLeak,
	"status_3xx":           CategoryInfoLeak,
	"status_4xx":           CategoryInfoLeak,
	"status_5xx":           CategoryInfoLeak,
	"is_html":              CategoryInfoLeak,
	"is_json":              CategoryInfoLeak,
	"has_error_indicators": CategoryInfoLeak,
	"num_error_indicators": CategoryInfoLeak,
	"num_framework_hints":  CategoryInfoLeak,

	// DOM rules
	"dom:inline-event-handler": CategoryDOMHygiene,
	"dom:javascript-href":      CategoryDOMHygiene,
	"dom:base-tag":             CategoryDOMHygiene,
	"dom:form-http-action":     CategoryDOMHygiene,
	"dom:iframe-src":           CategoryDOMHygiene,
	"dom:meta-refresh":         CategoryDOMHygiene,
	"dom:hardcoded-secret":     CategoryDOMHygiene,
	"dom:debug-artifact":       CategoryDOMHygiene,
	"dom:dev-comment":          CategoryDOMHygiene,
}

func CategoryForFeature(featureOrRuleID string) Category {
	if cat, ok := featureCategoryMap[featureOrRuleID]; ok {
		return cat
	}
	if strings.HasPrefix(featureOrRuleID, "dom:") {
		return CategoryDOMHygiene
	}
	return CategoryInfoLeak
}

func AllCategories() []Category {
	return []Category{
		CategoryHeaders,
		CategoryCookies,
		CategoryForms,
		CategoryParams,
		CategoryScripts,
		CategoryDOMHygiene,
		CategoryInfoLeak,
	}
}
