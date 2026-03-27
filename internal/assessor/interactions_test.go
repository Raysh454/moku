package assessor_test

import (
	"testing"

	"github.com/raysh454/moku/internal/assessor"
)

func TestEvaluateInteractions_NilRules_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	fired := assessor.EvaluateInteractions(nil, map[string]float64{}, map[string]float64{})
	if len(fired) != 0 {
		t.Errorf("expected 0 fired interactions, got %d", len(fired))
	}
}

func TestEvaluateInteractions_RequiresPresent_AllPresent_Fires(t *testing.T) {
	t.Parallel()

	rules := []assessor.InteractionRule{
		{
			ID:              "test:all-present",
			RequiresPresent: []string{"feat_a", "feat_b"},
			Boost:           0.1,
			TargetCategory:  assessor.CategoryForms,
		},
	}
	features := map[string]float64{"feat_a": 1.0, "feat_b": 1.0}

	fired := assessor.EvaluateInteractions(rules, features, map[string]float64{})
	if len(fired) != 1 {
		t.Fatalf("expected 1 fired interaction, got %d", len(fired))
	}
	if fired[0].Rule.ID != "test:all-present" {
		t.Errorf("expected rule ID test:all-present, got %q", fired[0].Rule.ID)
	}
}

func TestEvaluateInteractions_RequiresPresent_OneMissing_DoesNotFire(t *testing.T) {
	t.Parallel()

	rules := []assessor.InteractionRule{
		{
			ID:              "test:one-missing",
			RequiresPresent: []string{"feat_a", "feat_b"},
			Boost:           0.1,
			TargetCategory:  assessor.CategoryForms,
		},
	}
	features := map[string]float64{"feat_a": 1.0}

	fired := assessor.EvaluateInteractions(rules, features, map[string]float64{})
	if len(fired) != 0 {
		t.Errorf("expected 0 fired interactions, got %d", len(fired))
	}
}

func TestEvaluateInteractions_RequiresAbsent_FeaturePresent_DoesNotFire(t *testing.T) {
	t.Parallel()

	rules := []assessor.InteractionRule{
		{
			ID:              "test:absent-check",
			RequiresPresent: []string{"feat_a"},
			RequiresAbsent:  []string{"defense_b"},
			Boost:           0.1,
			TargetCategory:  assessor.CategoryForms,
		},
	}
	features := map[string]float64{"feat_a": 1.0, "defense_b": 1.0}

	fired := assessor.EvaluateInteractions(rules, features, map[string]float64{})
	if len(fired) != 0 {
		t.Errorf("expected 0 fired interactions when absent feature is present, got %d", len(fired))
	}
}

func TestEvaluateInteractions_UploadNoCsrf_Fires(t *testing.T) {
	t.Parallel()

	rules := assessor.DefaultInteractionRules()
	features := map[string]float64{
		"has_file_upload": 1.0,
	}

	fired := assessor.EvaluateInteractions(rules, features, map[string]float64{})

	foundUploadNoCsrf := false
	for _, f := range fired {
		if f.Rule.ID == "interaction:upload-no-csrf" {
			foundUploadNoCsrf = true
			if f.Boost != 0.15 {
				t.Errorf("expected boost 0.15, got %v", f.Boost)
			}
		}
	}
	if !foundUploadNoCsrf {
		t.Error("expected interaction:upload-no-csrf to fire")
	}
}

func TestEvaluateInteractions_UploadWithCsrf_DoesNotFire(t *testing.T) {
	t.Parallel()

	rules := assessor.DefaultInteractionRules()
	features := map[string]float64{
		"has_file_upload": 1.0,
		"has_csrf_input":  1.0,
	}

	fired := assessor.EvaluateInteractions(rules, features, map[string]float64{})

	for _, f := range fired {
		if f.Rule.ID == "interaction:upload-no-csrf" {
			t.Error("interaction:upload-no-csrf should NOT fire when CSRF input is present")
		}
	}
}

func TestDefaultInteractionRules_WellFormed(t *testing.T) {
	t.Parallel()

	rules := assessor.DefaultInteractionRules()
	if len(rules) == 0 {
		t.Fatal("DefaultInteractionRules returned empty slice")
	}

	ids := make(map[string]bool)
	for _, r := range rules {
		if r.ID == "" {
			t.Error("interaction rule has empty ID")
		}
		if ids[r.ID] {
			t.Errorf("duplicate interaction rule ID: %q", r.ID)
		}
		ids[r.ID] = true

		if len(r.RequiresPresent) == 0 {
			t.Errorf("interaction rule %q has no RequiresPresent", r.ID)
		}
		if r.Boost <= 0 {
			t.Errorf("interaction rule %q has non-positive boost: %v", r.ID, r.Boost)
		}
		if r.TargetCategory == "" {
			t.Errorf("interaction rule %q has empty TargetCategory", r.ID)
		}
	}
}

func TestEvaluateInteractions_LooksUpRuleContribs(t *testing.T) {
	t.Parallel()

	rules := []assessor.InteractionRule{
		{
			ID:              "test:from-contribs",
			RequiresPresent: []string{"dom:hardcoded-secret"},
			Boost:           0.2,
			TargetCategory:  assessor.CategoryDOMHygiene,
		},
	}
	features := map[string]float64{}
	ruleContribs := map[string]float64{"dom:hardcoded-secret": 0.4}

	fired := assessor.EvaluateInteractions(rules, features, ruleContribs)
	if len(fired) != 1 {
		t.Errorf("expected 1 fired interaction from ruleContribs lookup, got %d", len(fired))
	}
}
