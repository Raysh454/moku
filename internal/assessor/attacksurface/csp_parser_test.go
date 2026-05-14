package attacksurface

import (
	"testing"
)

func TestParseCSP_EmptyHeader(t *testing.T) {
	csp := ParseCSP("", false)
	if csp == nil {
		t.Fatal("expected non-nil CSPDirectives for empty header")
	} else {
		if csp.HasUnsafeInline {
			t.Error("expected HasUnsafeInline == false for empty CSP")
		}
		if csp.HasUnsafeEval {
			t.Error("expected HasUnsafeEval == false for empty CSP")
		}
		if csp.ReportOnly {
			t.Error("expected ReportOnly == false")
		}
	}
}

func TestParseCSP_FullRestrictive(t *testing.T) {
	header := "default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"
	csp := ParseCSP(header, false)

	if len(csp.DefaultSrc) == 0 {
		t.Error("expected DefaultSrc to be populated")
	}
	if csp.DefaultSrc[0] != "'none'" {
		t.Errorf("expected DefaultSrc[0] == 'none', got %q", csp.DefaultSrc[0])
	}
	if len(csp.ScriptSrc) == 0 || csp.ScriptSrc[0] != "'self'" {
		t.Errorf("expected ScriptSrc == ['self'], got %v", csp.ScriptSrc)
	}
	if len(csp.ObjectSrc) == 0 || csp.ObjectSrc[0] != "'none'" {
		t.Errorf("expected ObjectSrc == ['none'], got %v", csp.ObjectSrc)
	}
	if len(csp.BaseUri) == 0 || csp.BaseUri[0] != "'self'" {
		t.Errorf("expected BaseUri == ['self'], got %v", csp.BaseUri)
	}
	if len(csp.FormAction) == 0 || csp.FormAction[0] != "'self'" {
		t.Errorf("expected FormAction == ['self'], got %v", csp.FormAction)
	}
	if len(csp.FrameSrc) == 0 || csp.FrameSrc[0] != "'none'" {
		t.Errorf("expected FrameSrc == ['none'], got %v", csp.FrameSrc)
	}
	if csp.HasUnsafeInline {
		t.Error("expected HasUnsafeInline == false for restrictive CSP")
	}
	if csp.HasUnsafeEval {
		t.Error("expected HasUnsafeEval == false for restrictive CSP")
	}
}

func TestParseCSP_UnsafeInlineAndEval(t *testing.T) {
	header := "script-src 'self' 'unsafe-inline' 'unsafe-eval'; default-src 'self'"
	csp := ParseCSP(header, false)

	if !csp.HasUnsafeInline {
		t.Error("expected HasUnsafeInline == true")
	}
	if !csp.HasUnsafeEval {
		t.Error("expected HasUnsafeEval == true")
	}
}

func TestParseCSP_ReportOnly(t *testing.T) {
	header := "default-src 'self'; script-src 'self'"
	csp := ParseCSP(header, true)

	if !csp.ReportOnly {
		t.Error("expected ReportOnly == true")
	}
}

func TestParseCSP_PartialDirectives(t *testing.T) {
	header := "script-src 'self' https://cdn.example.com; object-src 'none'"
	csp := ParseCSP(header, false)

	if len(csp.ScriptSrc) != 2 {
		t.Fatalf("expected 2 ScriptSrc values, got %d: %v", len(csp.ScriptSrc), csp.ScriptSrc)
	}
	if len(csp.DefaultSrc) != 0 {
		t.Errorf("expected empty DefaultSrc for partial CSP, got %v", csp.DefaultSrc)
	}
}

func TestComputeCSPHardeningScore_EmptyCSP(t *testing.T) {
	csp := ParseCSP("", false)
	score := ComputeCSPHardeningScore(csp)
	if score != 0.0 {
		t.Errorf("expected 0.0 for empty CSP, got %v", score)
	}
}

func TestComputeCSPHardeningScore_FullRestrictiveCSP(t *testing.T) {
	header := "default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"
	csp := ParseCSP(header, false)
	score := ComputeCSPHardeningScore(csp)

	if score < 0.6 {
		t.Errorf("expected high score for full restrictive CSP, got %v", score)
	}
}

func TestComputeCSPHardeningScore_ReportOnlyReturnsZero(t *testing.T) {
	header := "default-src 'self'; script-src 'self'"
	csp := ParseCSP(header, true)
	score := ComputeCSPHardeningScore(csp)

	if score != 0.0 {
		t.Errorf("expected 0.0 for report-only CSP, got %v", score)
	}
}

func TestComputeCSPHardeningScore_UnsafeInlineReducesScore(t *testing.T) {
	restrictive := "default-src 'none'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"
	withUnsafe := "default-src 'none'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-src 'none'"

	scoreRestrict := ComputeCSPHardeningScore(ParseCSP(restrictive, false))
	scoreUnsafe := ComputeCSPHardeningScore(ParseCSP(withUnsafe, false))

	if scoreUnsafe >= scoreRestrict {
		t.Errorf("expected unsafe-inline CSP score (%v) < restrictive score (%v)", scoreUnsafe, scoreRestrict)
	}
}

func TestComputeCSPHardeningScore_NilCSP(t *testing.T) {
	score := ComputeCSPHardeningScore(nil)
	if score != 0.0 {
		t.Errorf("expected 0.0 for nil CSP, got %v", score)
	}
}
