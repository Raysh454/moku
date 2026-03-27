package assessor

type InteractionRule struct {
	ID              string   `json:"id" yaml:"id"`
	Description     string   `json:"description" yaml:"description"`
	RequiresPresent []string `json:"requires_present" yaml:"requires_present"`
	RequiresAbsent  []string `json:"requires_absent" yaml:"requires_absent"`
	Boost           float64  `json:"boost" yaml:"boost"`
	TargetCategory  Category `json:"target_category" yaml:"target_category"`
}

type FiredInteraction struct {
	Rule  InteractionRule `json:"rule"`
	Boost float64         `json:"boost"`
}

func DefaultInteractionRules() []InteractionRule {
	return []InteractionRule{
		{
			ID:              "interaction:upload-no-csrf",
			Description:     "File upload without CSRF token amplifies risk",
			RequiresPresent: []string{"has_file_upload"},
			RequiresAbsent:  []string{"has_csrf_input"},
			Boost:           0.15,
			TargetCategory:  CategoryForms,
		},
		{
			ID:              "interaction:password-no-hsts",
			Description:     "Password input on page missing HSTS",
			RequiresPresent: []string{"has_password_input", "hsts_missing"},
			Boost:           0.10,
			TargetCategory:  CategoryHeaders,
		},
		{
			ID:              "interaction:admin-form-no-csp",
			Description:     "Admin form on page missing CSP",
			RequiresPresent: []string{"has_admin_form", "csp_missing"},
			Boost:           0.12,
			TargetCategory:  CategoryForms,
		},
		{
			ID:              "interaction:inline-script-no-csp",
			Description:     "Inline scripts on page missing CSP",
			RequiresPresent: []string{"num_inline_scripts", "csp_missing"},
			Boost:           0.10,
			TargetCategory:  CategoryScripts,
		},
		{
			ID:              "interaction:session-cookie-no-hsts",
			Description:     "Session cookie without HttpOnly on page missing HSTS",
			RequiresPresent: []string{"has_session_cookie_no_httponly", "hsts_missing"},
			Boost:           0.10,
			TargetCategory:  CategoryCookies,
		},
	}
}

func EvaluateInteractions(rules []InteractionRule, features map[string]float64, ruleContribs map[string]float64) []FiredInteraction {
	if len(rules) == 0 {
		return nil
	}

	lookup := func(key string) float64 {
		if v, ok := features[key]; ok {
			return v
		}
		if v, ok := ruleContribs[key]; ok {
			return v
		}
		return 0
	}

	var fired []FiredInteraction
	for _, rule := range rules {
		if !allPresent(rule.RequiresPresent, lookup) {
			continue
		}
		if !allAbsent(rule.RequiresAbsent, lookup) {
			continue
		}
		fired = append(fired, FiredInteraction{
			Rule:  rule,
			Boost: rule.Boost,
		})
	}

	return fired
}

func allPresent(keys []string, lookup func(string) float64) bool {
	for _, k := range keys {
		if lookup(k) <= 0 {
			return false
		}
	}
	return true
}

func allAbsent(keys []string, lookup func(string) float64) bool {
	for _, k := range keys {
		if lookup(k) > 0 {
			return false
		}
	}
	return true
}
