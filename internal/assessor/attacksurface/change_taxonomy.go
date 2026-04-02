package attacksurface

// ChangeCategory classifies an attack surface change into a security-relevant category.
type ChangeCategory string

const (
	CategoryUploadSurface      ChangeCategory = "upload_surface"
	CategoryAuthSurface        ChangeCategory = "auth_surface"
	CategoryAdminSurface       ChangeCategory = "admin_surface"
	CategorySecurityRegression ChangeCategory = "security_regression"
	CategoryCookieRisk         ChangeCategory = "cookie_risk"
	CategoryCookieRegression   ChangeCategory = "cookie_regression"
	CategoryFormSurface        ChangeCategory = "form_surface"
	CategoryInputSurface       ChangeCategory = "input_surface"
	CategoryScriptSurface      ChangeCategory = "script_surface"
	CategoryParamSurface       ChangeCategory = "param_surface"
	CategoryGeneric            ChangeCategory = "generic"
)

// TaxonomyEntry pairs a category with a default score for a specific change kind.
type TaxonomyEntry struct {
	Category ChangeCategory
	Score    float64
}

// DefaultTaxonomyEntry is the fallback for unrecognized change kinds.
var DefaultTaxonomyEntry = TaxonomyEntry{Category: CategoryGeneric, Score: 0.05}

// ChangeTaxonomy maps "<kind>_<qualifier>" keys to their category and score.
// The qualifier comes from context (e.g., input type, form type, header name).
var ChangeTaxonomy = map[string]TaxonomyEntry{
	// Forms
	"form_added":        {CategoryFormSurface, 0.10},
	"form_added_admin":  {CategoryAdminSurface, 0.30},
	"form_added_auth":   {CategoryAuthSurface, 0.25},
	"form_added_upload": {CategoryUploadSurface, 0.30},
	"form_removed":      {CategoryFormSurface, 0.05},

	// Inputs
	"input_added":          {CategoryInputSurface, 0.05},
	"input_added_file":     {CategoryUploadSurface, 0.50},
	"input_added_password": {CategoryAuthSurface, 0.30},
	"input_removed":        {CategoryInputSurface, 0.02},
	"input_changed":        {CategoryInputSurface, 0.05},

	// Cookies
	"cookie_added":             {CategoryCookieRisk, 0.05},
	"cookie_added_no_httponly": {CategoryCookieRisk, 0.15},
	"cookie_added_no_secure":   {CategoryCookieRisk, 0.15},
	"cookie_removed":           {CategoryCookieRisk, 0.02},
	"cookie_httponly_removed":  {CategoryCookieRegression, 0.25},
	"cookie_secure_removed":    {CategoryCookieRegression, 0.25},
	"cookie_samesite_weakened": {CategoryCookieRegression, 0.15},
	"cookie_changed":           {CategoryCookieRisk, 0.05},

	// Scripts
	"script_added":   {CategoryScriptSurface, 0.05},
	"script_removed": {CategoryScriptSurface, 0.02},

	// Headers
	"header_added":                             {CategoryGeneric, 0.02},
	"header_removed":                           {CategoryGeneric, 0.02},
	"header_changed":                           {CategoryGeneric, 0.05},
	"header_changed_content-security-policy":   {CategorySecurityRegression, 0.20},
	"header_changed_strict-transport-security": {CategorySecurityRegression, 0.15},
	"header_changed_x-frame-options":           {CategorySecurityRegression, 0.10},
}

// ClassifyChange determines the category and score for a change kind with optional context qualifiers.
// Context keys: "input_type", "form_type", "header_name".
func ClassifyChange(kind string, context map[string]string) (ChangeCategory, float64) {
	// Try specific qualifier keys first
	if qualifier, ok := context["input_type"]; ok {
		key := kind + "_" + qualifier
		if entry, found := ChangeTaxonomy[key]; found {
			return entry.Category, entry.Score
		}
	}
	if qualifier, ok := context["form_type"]; ok {
		key := kind + "_" + qualifier
		if entry, found := ChangeTaxonomy[key]; found {
			return entry.Category, entry.Score
		}
	}
	if qualifier, ok := context["header_name"]; ok {
		key := kind + "_" + qualifier
		if entry, found := ChangeTaxonomy[key]; found {
			return entry.Category, entry.Score
		}
	}

	// Try the base kind
	if entry, found := ChangeTaxonomy[kind]; found {
		return entry.Category, entry.Score
	}

	return DefaultTaxonomyEntry.Category, DefaultTaxonomyEntry.Score
}

// SeverityForCategory returns a severity label for a change category.
func SeverityForCategory(cat ChangeCategory) string {
	switch cat {
	case CategoryUploadSurface, CategoryAdminSurface, CategorySecurityRegression, CategoryCookieRegression:
		return "high"
	case CategoryAuthSurface, CategoryCookieRisk:
		return "medium"
	default:
		return "low"
	}
}
