package attacksurface

import (
	"testing"
)

func TestComputeExposureScore_EmptyPage(t *testing.T) {
	as := &AttackSurface{}
	score := ComputeExposureScore(as, nil)
	if score != 0.0 {
		t.Errorf("expected 0.0 for empty page, got %v", score)
	}
}

func TestComputeExposureScore_NilAttackSurface(t *testing.T) {
	score := ComputeExposureScore(nil, nil)
	if score != 0.0 {
		t.Errorf("expected 0.0 for nil AttackSurface, got %v", score)
	}
}

func TestComputeExposureScore_LoginFormWithPasswordInput(t *testing.T) {
	as := &AttackSurface{
		Forms: []Form{
			{
				Action: "/login",
				Method: "POST",
				Inputs: []FormInput{
					{Name: "username", Type: "text"},
					{Name: "password", Type: "password"},
				},
			},
		},
	}
	score := ComputeExposureScore(as, nil)
	if score <= 0.0 {
		t.Errorf("expected positive score for login form, got %v", score)
	}
}

func TestComputeExposureScore_SaturationCapsScripts(t *testing.T) {
	scripts := make([]ScriptInfo, 50)
	for i := range scripts {
		scripts[i] = ScriptInfo{Inline: true, DOMIndex: i}
	}
	as := &AttackSurface{Scripts: scripts}

	satConfig := &SaturationConfig{
		Enabled: true,
		Caps:    map[string]float64{"script": 10},
	}

	scoreSaturated := ComputeExposureScore(as, satConfig)

	scoreUnsaturated := ComputeExposureScore(as, nil)

	if scoreUnsaturated <= scoreSaturated {
		t.Errorf("expected unsaturated score (%v) > saturated score (%v)", scoreUnsaturated, scoreSaturated)
	}
}

func TestComputeExposureScore_SaturationDisabledUsesRawCounts(t *testing.T) {
	scripts := make([]ScriptInfo, 50)
	for i := range scripts {
		scripts[i] = ScriptInfo{Inline: true, DOMIndex: i}
	}
	as := &AttackSurface{Scripts: scripts}

	disabledSat := &SaturationConfig{
		Enabled: false,
		Caps:    map[string]float64{"script": 10},
	}

	scoreDisabled := ComputeExposureScore(as, disabledSat)
	scoreNil := ComputeExposureScore(as, nil)

	if scoreDisabled != scoreNil {
		t.Errorf("expected disabled saturation (%v) == nil saturation (%v)", scoreDisabled, scoreNil)
	}
}

func TestComputeExposureScore_FileUploadScoresHigh(t *testing.T) {
	as := &AttackSurface{
		Forms: []Form{
			{
				Action: "/upload",
				Method: "POST",
				Inputs: []FormInput{
					{Name: "file", Type: "file"},
				},
			},
		},
	}
	score := ComputeExposureScore(as, nil)

	plainAs := &AttackSurface{
		Forms: []Form{
			{
				Action: "/search",
				Method: "GET",
				Inputs: []FormInput{
					{Name: "q", Type: "text"},
				},
			},
		},
	}
	plainScore := ComputeExposureScore(plainAs, nil)

	if score <= plainScore {
		t.Errorf("expected file upload score (%v) > plain form score (%v)", score, plainScore)
	}
}
