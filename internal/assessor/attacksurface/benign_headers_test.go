package attacksurface_test

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor/attacksurface"
)

func hasChangeKind(changes []attacksurface.AttackSurfaceChange, kind string) bool {
	for _, c := range changes {
		if c.Kind == kind {
			return true
		}
	}
	return false
}

func headerChangesOnly(changes []attacksurface.AttackSurfaceChange) []attacksurface.AttackSurfaceChange {
	var out []attacksurface.AttackSurfaceChange
	for _, c := range changes {
		if c.Kind == "header_added" || c.Kind == "header_removed" {
			out = append(out, c)
		}
	}
	return out
}

func TestDiffHeaders_BenignHeaderAddedIsFiltered(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"x-request-id": {"abc123"},
			"date":         {"Thu, 01 Jan 2026"},
			"cf-ray":       {"xyz"},
		},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 0 {
		t.Errorf("Expected no header changes for benign headers, got %d: %v", len(headerChanges), headerChanges)
	}
}

func TestDiffHeaders_BenignHeaderRemovedIsFiltered(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"x-cache": {"HIT"},
			"age":     {"300"},
		},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 0 {
		t.Errorf("Expected no header changes for benign header removal, got %d: %v", len(headerChanges), headerChanges)
	}
}

func TestDiffHeaders_SecurityHeaderNotFiltered(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"content-security-policy":   {"default-src 'self'"},
			"strict-transport-security": {"max-age=31536000"},
		},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 2 {
		t.Errorf("Expected 2 header_added changes for security headers, got %d: %v", len(headerChanges), headerChanges)
	}
}

func TestDiffHeaders_PrefixMatchFiltered(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"x-amz-cf-id":      {"some-id"},
			"x-cdn-node":       {"us-east-1"},
			"cf-connecting-ip": {"1.2.3.4"},
		},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 0 {
		t.Errorf("Expected no header changes for prefix-matched benign headers, got %d: %v", len(headerChanges), headerChanges)
	}
}

func TestDiffHeaders_MixedBenignAndSecurityHeaders(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"date":            {"Thu, 01 Jan 2026"},
			"server":          {"nginx"},
			"x-request-id":    {"abc"},
			"x-frame-options": {"DENY"},
			"referrer-policy": {"no-referrer"},
		},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 2 {
		t.Errorf("Expected 2 security header changes, got %d: %v", len(headerChanges), headerChanges)
	}

	foundXFO := false
	foundRP := false
	for _, c := range headerChanges {
		if c.Detail == "Header added: x-frame-options" {
			foundXFO = true
		}
		if c.Detail == "Header added: referrer-policy" {
			foundRP = true
		}
	}
	if !foundXFO {
		t.Error("Expected x-frame-options to be reported")
	}
	if !foundRP {
		t.Error("Expected referrer-policy to be reported")
	}
}

func TestDiffHeaders_UnknownCustomHeaderNotFiltered(t *testing.T) {
	base := &attacksurface.AttackSurface{
		Headers: map[string][]string{},
	}
	head := &attacksurface.AttackSurface{
		Headers: map[string][]string{
			"x-custom-auth-method": {"bearer"},
			"x-powered-by":         {"Express"},
		},
	}

	changes := attacksurface.DiffAttackSurfaces(base, head)
	headerChanges := headerChangesOnly(changes)

	if len(headerChanges) != 2 {
		t.Errorf("Expected 2 header_added changes for unknown custom headers, got %d: %v", len(headerChanges), headerChanges)
	}
}
