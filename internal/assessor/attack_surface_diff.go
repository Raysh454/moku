package assessor

import (
	"fmt"
	"strconv"
	"strings"
)

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

	changes = append(changes, diffForms(base, head)...)
	changes = append(changes, diffFormInputs(base, head)...)
	changes = append(changes, diffCookies(base, head)...)
	changes = append(changes, diffScripts(base, head)...)
	changes = append(changes, diffHeaders(base, head)...)
	changes = append(changes, diffSecurityHeaders(base, head)...)

	return changes
}

func diffForms(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

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

	return changes
}

func diffFormInputs(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

	baseFormsByKey := make(map[string]Form)
	for _, form := range base.Forms {
		key := form.Action + ":" + form.Method
		baseFormsByKey[key] = form
	}

	headFormsByKey := make(map[string]Form)
	for _, form := range head.Forms {
		key := form.Action + ":" + form.Method
		headFormsByKey[key] = form
	}

	for key, baseForm := range baseFormsByKey {
		headForm, ok := headFormsByKey[key]
		if !ok {
			continue
		}

		baseInputsByName := make(map[string]FormInput)
		for _, input := range baseForm.Inputs {
			if input.Name == "" {
				continue
			}
			baseInputsByName[input.Name] = input
		}

		headInputsByName := make(map[string]FormInput)
		for _, input := range headForm.Inputs {
			if input.Name == "" {
				continue
			}
			headInputsByName[input.Name] = input
		}

		for name, headInput := range headInputsByName {
			baseInput, exists := baseInputsByName[name]
			if !exists {
				changes = append(changes, AttackSurfaceChange{
					Kind:   "input_added",
					Detail: fmt.Sprintf("Input added to form %s %s: %s (%s, required=%t)", headForm.Method, headForm.Action, headInput.Name, headInput.Type, headInput.Required),
				})
				continue
			}

			if change := compareFormInput(headForm.Action, headForm.Method, baseInput, headInput); change != nil {
				changes = append(changes, *change)
			}
		}

		for name, baseInput := range baseInputsByName {
			if _, exists := headInputsByName[name]; !exists {
				changes = append(changes, AttackSurfaceChange{
					Kind:   "input_removed",
					Detail: fmt.Sprintf("Input removed from form %s %s: %s (%s, required=%t)", baseForm.Method, baseForm.Action, baseInput.Name, baseInput.Type, baseInput.Required),
				})
			}
		}
	}

	return changes
}

func compareFormInput(formAction, formMethod string, base, head FormInput) *AttackSurfaceChange {
	var detailParts []string

	if base.Type != head.Type {
		detailParts = append(detailParts, fmt.Sprintf("type %q -> %q", base.Type, head.Type))
	}
	if base.Required != head.Required {
		detailParts = append(detailParts, fmt.Sprintf("required %t -> %t", base.Required, head.Required))
	}

	if len(detailParts) == 0 {
		return nil
	}

	return &AttackSurfaceChange{
		Kind:   "input_changed",
		Detail: fmt.Sprintf("Input %q in form %s %s changed: %s", base.Name, formMethod, formAction, strings.Join(detailParts, ", ")),
	}
}

func diffCookies(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

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

	return changes
}

func diffScripts(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

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

	return changes
}

func diffHeaders(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

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

	return changes
}

func diffSecurityHeaders(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

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
