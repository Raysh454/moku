package assessor

// DefaultRules returns the built-in set of DOM-based security heuristic rules.
// CSS selector rules target structural patterns; regex rules target textual patterns.
func DefaultRules() []Rule {
	return []Rule{
		// --- CSS selector rules ---
		{
			ID:       "dom:inline-event-handler",
			Key:      "inline-event-handler",
			Severity: "high",
			Weight:   0.3,
			Selector: "[onclick],[onerror],[onload],[onmouseover],[onfocus],[onblur],[onsubmit]",
		},
		{
			ID:       "dom:javascript-href",
			Key:      "javascript-href",
			Severity: "high",
			Weight:   0.3,
			Selector: `a[href^="javascript:"]`,
		},
		{
			ID:       "dom:base-tag",
			Key:      "base-tag",
			Severity: "high",
			Weight:   0.25,
			Selector: "base[href]",
		},
		{
			ID:       "dom:form-http-action",
			Key:      "form-http-action",
			Severity: "medium",
			Weight:   0.2,
			Selector: `form[action^="http:"]`,
		},
		{
			ID:       "dom:iframe-src",
			Key:      "iframe-src",
			Severity: "medium",
			Weight:   0.15,
			Selector: "iframe[src]",
		},
		{
			ID:       "dom:meta-refresh",
			Key:      "meta-refresh",
			Severity: "medium",
			Weight:   0.15,
			Selector: `meta[http-equiv="refresh"]`,
		},

		// --- Regex rules ---
		{
			ID:       "dom:hardcoded-secret",
			Key:      "hardcoded-secret",
			Severity: "critical",
			Weight:   0.4,
			Regex:    `(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*["'][^"']+["']`,
		},
		{
			ID:       "dom:debug-artifact",
			Key:      "debug-artifact",
			Severity: "medium",
			Weight:   0.1,
			Regex:    `(?i)(//\s*debug|console\.log|console\.debug)`,
		},
		{
			ID:       "dom:dev-comment",
			Key:      "dev-comment",
			Severity: "low",
			Weight:   0.05,
			Regex:    `(?i)<!--\s*(TODO|FIXME|HACK|BUG|XXX)`,
		},
	}
}
