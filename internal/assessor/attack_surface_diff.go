package assessor

import "strconv"

// AttackSurfaceChange represents a specific change in the attack surface between two versions.
type AttackSurfaceChange struct {
	Kind   string `json:"kind"`   // e.g., "form_added", "input_added", "header_changed", "cookie_changed", "script_added"
	Detail string `json:"detail"` // human-readable description of the change
}

// DiffAttackSurfaces compares two AttackSurface instances and returns a list of changes.
// This provides a high-level summary of what changed on the page between versions.
func DiffAttackSurfaces(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

	if base == nil && head == nil {
		return changes
	}

	// Handle nil cases
	if base == nil {
		base = &AttackSurface{}
	}
	if head == nil {
		head = &AttackSurface{}
	}

	// Compare forms
	baseFormKeys := make(map[string]bool)
	for _, form := range base.Forms {
		key := form.Action + ":" + form.Method
		baseFormKeys[key] = true
	}

	headFormKeys := make(map[string]bool)
	for _, form := range head.Forms {
		key := form.Action + ":" + form.Method
		headFormKeys[key] = true

		if !baseFormKeys[key] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "form_added",
				Detail: "Form added: " + form.Method + " " + form.Action,
			})
		}
	}

	for _, form := range base.Forms {
		key := form.Action + ":" + form.Method
		if !headFormKeys[key] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "form_removed",
				Detail: "Form removed: " + form.Method + " " + form.Action,
			})
		}
	}

	// Compare form inputs (simplified: just count changes)
	baseInputCount := 0
	for _, form := range base.Forms {
		baseInputCount += len(form.Inputs)
	}
	headInputCount := 0
	for _, form := range head.Forms {
		headInputCount += len(form.Inputs)
	}
	if headInputCount > baseInputCount {
		changes = append(changes, AttackSurfaceChange{
			Kind:   "input_added",
			Detail: "Form inputs increased from " + strconv.Itoa(baseInputCount) + " to " + strconv.Itoa(headInputCount),
		})
	} else if headInputCount < baseInputCount {
		changes = append(changes, AttackSurfaceChange{
			Kind:   "input_removed",
			Detail: "Form inputs decreased from " + strconv.Itoa(baseInputCount) + " to " + strconv.Itoa(headInputCount),
		})
	}

	// Compare cookies
	baseCookieNames := make(map[string]bool)
	for _, cookie := range base.Cookies {
		baseCookieNames[cookie.Name] = true
	}

	for _, cookie := range head.Cookies {
		if !baseCookieNames[cookie.Name] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "cookie_added",
				Detail: "Cookie added: " + cookie.Name,
			})
		}
	}

	headCookieNames := make(map[string]bool)
	for _, cookie := range head.Cookies {
		headCookieNames[cookie.Name] = true
	}

	for _, cookie := range base.Cookies {
		if !headCookieNames[cookie.Name] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "cookie_removed",
				Detail: "Cookie removed: " + cookie.Name,
			})
		}
	}

	// Compare scripts (just count)
	if len(head.Scripts) > len(base.Scripts) {
		changes = append(changes, AttackSurfaceChange{
			Kind:   "script_added",
			Detail: "Scripts increased from " + strconv.Itoa(len(base.Scripts)) + " to " + strconv.Itoa(len(head.Scripts)),
		})
	} else if len(head.Scripts) < len(base.Scripts) {
		changes = append(changes, AttackSurfaceChange{
			Kind:   "script_removed",
			Detail: "Scripts decreased from " + strconv.Itoa(len(base.Scripts)) + " to " + strconv.Itoa(len(head.Scripts)),
		})
	}

	// Compare headers (simplified: just check for new/removed headers)
	baseHeaderKeys := make(map[string]bool)
	for key := range base.Headers {
		baseHeaderKeys[key] = true
	}

	for key := range head.Headers {
		if !baseHeaderKeys[key] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "header_added",
				Detail: "Header added: " + key,
			})
		}
	}

	headHeaderKeys := make(map[string]bool)
	for key := range head.Headers {
		headHeaderKeys[key] = true
	}

	for key := range base.Headers {
		if !headHeaderKeys[key] {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "header_removed",
				Detail: "Header removed: " + key,
			})
		}
	}

	// Check for header value changes (security-relevant headers)
	securityHeaders := []string{
		"content-security-policy",
		"x-frame-options",
		"x-content-type-options",
		"strict-transport-security",
		"x-xss-protection",
	}

	for _, header := range securityHeaders {
		baseVal, baseExists := base.Headers[header]
		headVal, headExists := head.Headers[header]

		if baseExists && headExists && baseVal != headVal {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "header_changed",
				Detail: "Security header changed: " + header,
			})
		}
	}

	return changes
}
