package attacksurface

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Minimal EvidenceLocation struct to avoid circular import
type EvidenceLocation struct {
	Type           string `json:"type"` // "form","input","header","cookie","script",...
	SnapshotID     string `json:"snapshot_id,omitempty"`
	DOMIndex       *int   `json:"dom_index,omitempty"`        // index into getElementsByTagName for this Type
	ParentDOMIndex *int   `json:"parent_dom_index,omitempty"` // index of parent form for inputs

	HeaderName string `json:"header_name,omitempty"`
	CookieName string `json:"cookie_name,omitempty"`

	ParamName string `json:"param_name,omitempty"`
}

// AttackSurfaceChange represents a specific change in the attack surface between two versions.
type AttackSurfaceChange struct {
	Kind      string             `json:"kind"`   // e.g., "form_added", "input_added", "header_changed", "cookie_changed", "script_added"
	Detail    string             `json:"detail"` // human-readable description of the change
	Locations []EvidenceLocation `json:"evidence_locations,omitempty"`
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
			idx := form.DOMIndex
			changes = append(changes, AttackSurfaceChange{
				Kind:   "form_added",
				Detail: "Form added: " + form.Method + " " + form.Action,
				Locations: []EvidenceLocation{
					{
						Type:       "form",
						SnapshotID: head.SnapshotID,
						DOMIndex:   &idx,
					},
				},
			})
		}
	}

	for _, form := range base.Forms {
		key := form.Action + ":" + form.Method
		if !headFormKeys[key] {
			idx := form.DOMIndex
			changes = append(changes, AttackSurfaceChange{
				Kind:   "form_removed",
				Detail: "Form removed: " + form.Method + " " + form.Action,
				Locations: []EvidenceLocation{
					{
						Type:       "form",
						SnapshotID: base.SnapshotID,
						DOMIndex:   &idx,
					},
				},
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
				fIdx := headForm.DOMIndex
				iIdx := headInput.DOMIndex
				changes = append(changes, AttackSurfaceChange{
					Kind:   "input_added",
					Detail: fmt.Sprintf("Input added to form %s %s: %s (%s, required=%t)", headForm.Method, headForm.Action, headInput.Name, headInput.Type, headInput.Required),
					Locations: []EvidenceLocation{
						{
							Type:           "input",
							SnapshotID:     head.SnapshotID,
							ParentDOMIndex: &fIdx,
							DOMIndex:       &iIdx,
						},
					},
				})
				continue
			}

			if change := compareFormInput(headForm.Action, headForm.Method, baseInput, headInput); change != nil {
				// attach location to the existing change
				fIdx := headForm.DOMIndex
				iIdx := headInput.DOMIndex
				change.Locations = []EvidenceLocation{
					{
						Type:           "input",
						SnapshotID:     head.SnapshotID,
						ParentDOMIndex: &fIdx,
						DOMIndex:       &iIdx,
					},
				}
				changes = append(changes, *change)
			}
		}

		for name, baseInput := range baseInputsByName {
			if _, exists := headInputsByName[name]; !exists {
				fIdx := baseForm.DOMIndex
				iIdx := baseInput.DOMIndex
				changes = append(changes, AttackSurfaceChange{
					Kind:   "input_removed",
					Detail: fmt.Sprintf("Input removed from form %s %s: %s (%s, required=%t)", baseForm.Method, baseForm.Action, baseInput.Name, baseInput.Type, baseInput.Required),
					Locations: []EvidenceLocation{
						{
							Type:           "input",
							SnapshotID:     base.SnapshotID,
							ParentDOMIndex: &fIdx,
							DOMIndex:       &iIdx,
						},
					},
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
				Locations: []EvidenceLocation{
					{
						Type:       "cookie",
						CookieName: cookie.Name,
						SnapshotID: head.SnapshotID,
					},
				},
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
				Locations: []EvidenceLocation{
					{
						Type:       "cookie",
						CookieName: cookie.Name,
						SnapshotID: base.SnapshotID,
					},
				},
			})
		}
	}

	return changes
}

func diffScripts(base, head *AttackSurface) []AttackSurfaceChange {
	changes := []AttackSurfaceChange{}

	// If you only care about count changes, but still want locations, you can
	// gather locations for all "new" or "removed" scripts.

	if len(head.Scripts) > len(base.Scripts) {
		// Collect locations for scripts that exist in head but not in base.
		// Simple heuristic: compare by Src when available, otherwise by DOMIndex.
		baseBySrc := make(map[string]bool)
		for _, s := range base.Scripts {
			if s.Src != "" {
				baseBySrc[s.Src] = true
			}
		}

		var locs []EvidenceLocation
		for _, s := range head.Scripts {
			// If we have a Src and it wasn't in base, treat as added
			if s.Src != "" && !baseBySrc[s.Src] {
				idx := s.DOMIndex
				locs = append(locs, EvidenceLocation{
					Type:       "script",
					SnapshotID: head.SnapshotID,
					DOMIndex:   &idx, // index into document.getElementsByTagName("script")
				})
			}
		}

		changes = append(changes, AttackSurfaceChange{
			Kind:      "script_added",
			Detail:    "Scripts increased from " + strconv.Itoa(len(base.Scripts)) + " to " + strconv.Itoa(len(head.Scripts)),
			Locations: locs,
		})

	} else if len(head.Scripts) < len(base.Scripts) {
		// Collect locations for scripts that existed in base but not in head.
		headBySrc := make(map[string]bool)
		for _, s := range head.Scripts {
			if s.Src != "" {
				headBySrc[s.Src] = true
			}
		}

		var locs []EvidenceLocation
		for _, s := range base.Scripts {
			if s.Src != "" && !headBySrc[s.Src] {
				idx := s.DOMIndex
				locs = append(locs, EvidenceLocation{
					Type:       "script",
					SnapshotID: base.SnapshotID,
					DOMIndex:   &idx,
				})
			}
		}

		changes = append(changes, AttackSurfaceChange{
			Kind:      "script_removed",
			Detail:    "Scripts decreased from " + strconv.Itoa(len(base.Scripts)) + " to " + strconv.Itoa(len(head.Scripts)),
			Locations: locs,
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
				Locations: []EvidenceLocation{
					{
						Type:       "header",
						HeaderName: key,
						SnapshotID: head.SnapshotID,
					},
				},
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
				Locations: []EvidenceLocation{
					{
						Type:       "header",
						HeaderName: key,
						SnapshotID: base.SnapshotID,
					},
				},
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

		if baseExists && headExists && !slices.Equal(baseVal, headVal) {
			changes = append(changes, AttackSurfaceChange{
				Kind:   "header_changed",
				Detail: "Security header changed: " + header,
				Locations: []EvidenceLocation{
					{
						Type:       "header",
						HeaderName: header,
						SnapshotID: head.SnapshotID,
					},
				},
			})
		}
	}

	return changes
}
