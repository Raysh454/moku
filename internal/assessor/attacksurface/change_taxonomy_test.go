package attacksurface

import "testing"

func TestClassifyChange_UploadInputAdded(t *testing.T) {
	cat, score := ClassifyChange("input_added", map[string]string{"input_type": "file"})
	if cat != CategoryUploadSurface {
		t.Errorf("expected CategoryUploadSurface, got %q", cat)
	}
	if score != 0.50 {
		t.Errorf("expected score 0.50, got %v", score)
	}
}

func TestClassifyChange_AuthFormAdded(t *testing.T) {
	cat, score := ClassifyChange("form_added", map[string]string{"form_type": "auth"})
	if cat != CategoryAuthSurface {
		t.Errorf("expected CategoryAuthSurface, got %q", cat)
	}
	if score != 0.25 {
		t.Errorf("expected score 0.25, got %v", score)
	}
}

func TestClassifyChange_AdminFormAdded(t *testing.T) {
	cat, score := ClassifyChange("form_added", map[string]string{"form_type": "admin"})
	if cat != CategoryAdminSurface {
		t.Errorf("expected CategoryAdminSurface, got %q", cat)
	}
	if score != 0.30 {
		t.Errorf("expected score 0.30, got %v", score)
	}
}

func TestClassifyChange_GenericChange(t *testing.T) {
	cat, score := ClassifyChange("form_added", nil)
	if cat != CategoryFormSurface {
		t.Errorf("expected CategoryFormSurface, got %q", cat)
	}
	if score != 0.10 {
		t.Errorf("expected score 0.10, got %v", score)
	}
}

func TestClassifyChange_UnknownKindFallback(t *testing.T) {
	cat, score := ClassifyChange("unknown_thing", nil)
	if cat != CategoryGeneric {
		t.Errorf("expected CategoryGeneric, got %q", cat)
	}
	if score != 0.05 {
		t.Errorf("expected score 0.05, got %v", score)
	}
}

func TestClassifyChange_HeaderChangedCSP(t *testing.T) {
	cat, score := ClassifyChange("header_changed", map[string]string{"header_name": "content-security-policy"})
	if cat != CategorySecurityRegression {
		t.Errorf("expected CategorySecurityRegression, got %q", cat)
	}
	if score != 0.20 {
		t.Errorf("expected score 0.20, got %v", score)
	}
}

func TestClassifyChange_CookieHttpOnlyRemoved(t *testing.T) {
	cat, score := ClassifyChange("cookie_httponly_removed", nil)
	if cat != CategoryCookieRegression {
		t.Errorf("expected CategoryCookieRegression, got %q", cat)
	}
	if score != 0.25 {
		t.Errorf("expected score 0.25, got %v", score)
	}
}

func TestSeverityForCategory_High(t *testing.T) {
	highCats := []ChangeCategory{CategoryUploadSurface, CategoryAdminSurface, CategorySecurityRegression, CategoryCookieRegression}
	for _, cat := range highCats {
		if got := SeverityForCategory(cat); got != "high" {
			t.Errorf("SeverityForCategory(%q) = %q, want high", cat, got)
		}
	}
}

func TestSeverityForCategory_Medium(t *testing.T) {
	medCats := []ChangeCategory{CategoryAuthSurface, CategoryCookieRisk}
	for _, cat := range medCats {
		if got := SeverityForCategory(cat); got != "medium" {
			t.Errorf("SeverityForCategory(%q) = %q, want medium", cat, got)
		}
	}
}

func TestSeverityForCategory_Low(t *testing.T) {
	lowCats := []ChangeCategory{CategoryFormSurface, CategoryInputSurface, CategoryScriptSurface, CategoryGeneric}
	for _, cat := range lowCats {
		if got := SeverityForCategory(cat); got != "low" {
			t.Errorf("SeverityForCategory(%q) = %q, want low", cat, got)
		}
	}
}
