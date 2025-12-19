package assessor

import (
	"testing"
	"time"
)

func TestDiffAttackSurfaces_BothNil(t *testing.T) {
	changes := DiffAttackSurfaces(nil, nil)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes for nil attack surfaces, got %d", len(changes))
	}
}

func TestDiffAttackSurfaces_FormAdded(t *testing.T) {
	base := &AttackSurface{
		URL:         "https://example.com",
		CollectedAt: time.Now(),
		Forms:       []Form{},
	}

	head := &AttackSurface{
		URL:         "https://example.com",
		CollectedAt: time.Now(),
		Forms: []Form{
			{Action: "/submit", Method: "POST"},
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundFormAdded := false
	for _, change := range changes {
		if change.Kind == "form_added" {
			foundFormAdded = true
		}
	}

	if !foundFormAdded {
		t.Error("Expected to find form_added change")
	}
}

func TestDiffAttackSurfaces_FormRemoved(t *testing.T) {
	base := &AttackSurface{
		Forms: []Form{
			{Action: "/login", Method: "POST"},
		},
	}

	head := &AttackSurface{
		Forms: []Form{},
	}

	changes := DiffAttackSurfaces(base, head)

	foundFormRemoved := false
	for _, change := range changes {
		if change.Kind == "form_removed" {
			foundFormRemoved = true
		}
	}

	if !foundFormRemoved {
		t.Error("Expected to find form_removed change")
	}
}

func TestDiffAttackSurfaces_InputsChanged(t *testing.T) {
	base := &AttackSurface{
		Forms: []Form{
			{
				Action: "/submit",
				Method: "POST",
				Inputs: []FormInput{
					{Name: "username", Type: "text"},
				},
			},
		},
	}

	head := &AttackSurface{
		Forms: []Form{
			{
				Action: "/submit",
				Method: "POST",
				Inputs: []FormInput{
					{Name: "username", Type: "text"},
					{Name: "password", Type: "password"},
					{Name: "remember", Type: "checkbox"},
				},
			},
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundInputAdded := false
	for _, change := range changes {
		if change.Kind == "input_added" {
			foundInputAdded = true
		}
	}

	if !foundInputAdded {
		t.Error("Expected to find input_added change")
	}
}

func TestDiffAttackSurfaces_CookieAdded(t *testing.T) {
	base := &AttackSurface{
		Cookies: []CookieInfo{},
	}

	head := &AttackSurface{
		Cookies: []CookieInfo{
			{Name: "session", Secure: true, HttpOnly: true},
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundCookieAdded := false
	for _, change := range changes {
		if change.Kind == "cookie_added" && change.Detail == "Cookie added: session" {
			foundCookieAdded = true
		}
	}

	if !foundCookieAdded {
		t.Error("Expected to find cookie_added change for session")
	}
}

func TestDiffAttackSurfaces_ScriptAdded(t *testing.T) {
	base := &AttackSurface{
		Scripts: []ScriptInfo{},
	}

	head := &AttackSurface{
		Scripts: []ScriptInfo{
			{Src: "https://cdn.example.com/app.js", Inline: false},
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundScriptAdded := false
	for _, change := range changes {
		if change.Kind == "script_added" {
			foundScriptAdded = true
		}
	}

	if !foundScriptAdded {
		t.Error("Expected to find script_added change")
	}
}

func TestDiffAttackSurfaces_HeaderAdded(t *testing.T) {
	base := &AttackSurface{
		Headers: map[string]string{},
	}

	head := &AttackSurface{
		Headers: map[string]string{
			"x-frame-options": "DENY",
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundHeaderAdded := false
	for _, change := range changes {
		if change.Kind == "header_added" && change.Detail == "Header added: x-frame-options" {
			foundHeaderAdded = true
		}
	}

	if !foundHeaderAdded {
		t.Error("Expected to find header_added change for x-frame-options")
	}
}

func TestDiffAttackSurfaces_SecurityHeaderChanged(t *testing.T) {
	base := &AttackSurface{
		Headers: map[string]string{
			"content-security-policy": "default-src 'self'",
		},
	}

	head := &AttackSurface{
		Headers: map[string]string{
			"content-security-policy": "default-src 'self'; script-src 'unsafe-inline'",
		},
	}

	changes := DiffAttackSurfaces(base, head)

	foundHeaderChanged := false
	for _, change := range changes {
		if change.Kind == "header_changed" && change.Detail == "Security header changed: content-security-policy" {
			foundHeaderChanged = true
		}
	}

	if !foundHeaderChanged {
		t.Error("Expected to find header_changed for content-security-policy")
	}
}

func TestDiffAttackSurfaces_MultipleChanges(t *testing.T) {
	base := &AttackSurface{
		Forms: []Form{
			{Action: "/login", Method: "POST"},
		},
		Cookies: []CookieInfo{
			{Name: "old-cookie"},
		},
		Scripts: []ScriptInfo{},
		Headers: map[string]string{
			"x-frame-options": "SAMEORIGIN",
		},
	}

	head := &AttackSurface{
		Forms: []Form{
			{Action: "/login", Method: "POST"},
			{Action: "/register", Method: "POST"},
		},
		Cookies: []CookieInfo{
			{Name: "new-cookie"},
		},
		Scripts: []ScriptInfo{
			{Src: "app.js"},
		},
		Headers: map[string]string{
			"x-frame-options":         "DENY",
			"content-security-policy": "default-src 'self'",
		},
	}

	changes := DiffAttackSurfaces(base, head)

	if len(changes) == 0 {
		t.Error("Expected multiple changes, got none")
	}

	changeKinds := make(map[string]bool)
	for _, change := range changes {
		changeKinds[change.Kind] = true
	}

	// We expect form_added, cookie changes, script_added, header changes
	expectedKinds := []string{"form_added", "script_added", "header_added", "header_changed"}
	for _, kind := range expectedKinds {
		if !changeKinds[kind] {
			t.Logf("Changes found: %v", changeKinds)
			t.Errorf("Expected to find change kind: %s", kind)
		}
	}
}
