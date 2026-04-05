package filter

import (
	"reflect"
	"testing"
)

func TestDefaultFilterConfig(t *testing.T) {
	config := DefaultFilterConfig()

	// Check that security defaults are set
	if len(config.SkipExtensions) == 0 {
		t.Error("DefaultFilterConfig() should have SkipExtensions")
	}

	// Verify security-important extensions are NOT in skip list
	securityExtensions := []string{".js", ".json", ".xml", ".html", ".php", ".svg", ".env", ".config"}
	for _, ext := range securityExtensions {
		for _, skip := range config.SkipExtensions {
			if skip == ext {
				t.Errorf("DefaultFilterConfig() should not skip security-relevant extension %s", ext)
			}
		}
	}

	// Verify binary extensions ARE in skip list
	binaryExtensions := []string{".jpg", ".png", ".gif", ".mp4", ".zip", ".exe"}
	for _, ext := range binaryExtensions {
		found := false
		for _, skip := range config.SkipExtensions {
			if skip == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultFilterConfig() should skip binary extension %s", ext)
		}
	}

	// Status codes should be empty by default (404 is opt-in)
	if len(config.SkipStatusCodes) != 0 {
		t.Errorf("DefaultFilterConfig() should not skip any status codes by default, got %v", config.SkipStatusCodes)
	}
}

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name   string
		global *FilterConfig
		site   *FilterConfig
		api    *FilterConfig
		want   *FilterConfig
	}{
		{
			name:   "all nil",
			global: nil,
			site:   nil,
			api:    nil,
			want:   &FilterConfig{},
		},
		{
			name: "only global",
			global: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png"},
			},
			site: nil,
			api:  nil,
			want: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png"},
			},
		},
		{
			name: "global + site merge",
			global: &FilterConfig{
				SkipExtensions: []string{".jpg"},
			},
			site: &FilterConfig{
				SkipExtensions: []string{".png"},
			},
			api: nil,
			want: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png"},
			},
		},
		{
			name: "all three levels merge",
			global: &FilterConfig{
				SkipExtensions: []string{".jpg"},
			},
			site: &FilterConfig{
				SkipPatterns: []string{"*/media/*"},
			},
			api: &FilterConfig{
				SkipStatusCodes: []int{404},
			},
			want: &FilterConfig{
				SkipExtensions:  []string{".jpg"},
				SkipPatterns:    []string{"*/media/*"},
				SkipStatusCodes: []int{404},
			},
		},
		{
			name: "deduplication",
			global: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png"},
			},
			site: &FilterConfig{
				SkipExtensions: []string{".png", ".gif"}, // .png is duplicate
			},
			api: nil,
			want: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png", ".gif"},
			},
		},
		{
			name: "status codes merge",
			global: &FilterConfig{
				SkipStatusCodes: []int{404},
			},
			site: &FilterConfig{
				SkipStatusCodes: []int{410},
			},
			api: &FilterConfig{
				SkipStatusCodes: []int{500},
			},
			want: &FilterConfig{
				SkipStatusCodes: []int{404, 410, 500},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeConfigs(tt.global, tt.site, tt.api)

			// Compare SkipExtensions
			if !stringSliceEqual(got.SkipExtensions, tt.want.SkipExtensions) {
				t.Errorf("MergeConfigs() SkipExtensions = %v, want %v", got.SkipExtensions, tt.want.SkipExtensions)
			}

			// Compare SkipPatterns
			if !stringSliceEqual(got.SkipPatterns, tt.want.SkipPatterns) {
				t.Errorf("MergeConfigs() SkipPatterns = %v, want %v", got.SkipPatterns, tt.want.SkipPatterns)
			}

			// Compare SkipStatusCodes
			if !intSliceEqual(got.SkipStatusCodes, tt.want.SkipStatusCodes) {
				t.Errorf("MergeConfigs() SkipStatusCodes = %v, want %v", got.SkipStatusCodes, tt.want.SkipStatusCodes)
			}
		})
	}
}

func TestRulesToConfig(t *testing.T) {
	rules := []FilterRule{
		{RuleType: RuleTypeExtension, RuleValue: ".jpg", Enabled: true},
		{RuleType: RuleTypeExtension, RuleValue: ".png", Enabled: true},
		{RuleType: RuleTypeExtension, RuleValue: ".gif", Enabled: false}, // disabled
		{RuleType: RuleTypePattern, RuleValue: "*/media/*", Enabled: true},
		{RuleType: RuleTypeStatusCode, RuleValue: "404", Enabled: true},
		{RuleType: RuleTypeStatusCode, RuleValue: "500", Enabled: true},
	}

	config := RulesToConfig(rules)

	expectedExt := []string{".jpg", ".png"}
	if !stringSliceEqual(config.SkipExtensions, expectedExt) {
		t.Errorf("RulesToConfig() SkipExtensions = %v, want %v", config.SkipExtensions, expectedExt)
	}

	expectedPatterns := []string{"*/media/*"}
	if !stringSliceEqual(config.SkipPatterns, expectedPatterns) {
		t.Errorf("RulesToConfig() SkipPatterns = %v, want %v", config.SkipPatterns, expectedPatterns)
	}

	expectedStatus := []int{404, 500}
	if !intSliceEqual(config.SkipStatusCodes, expectedStatus) {
		t.Errorf("RulesToConfig() SkipStatusCodes = %v, want %v", config.SkipStatusCodes, expectedStatus)
	}
}

func TestRulesToConfig_Empty(t *testing.T) {
	config := RulesToConfig([]FilterRule{})
	if !config.IsEmpty() {
		t.Error("RulesToConfig([]) should return empty config")
	}
}

func TestRulesToConfig_InvalidStatusCode(t *testing.T) {
	rules := []FilterRule{
		{RuleType: RuleTypeStatusCode, RuleValue: "invalid", Enabled: true},
	}

	config := RulesToConfig(rules)
	if len(config.SkipStatusCodes) != 0 {
		t.Error("RulesToConfig() should skip invalid status codes")
	}
}

// Helper functions
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}
	return true
}

func intSliceEqual(a, b []int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	return reflect.DeepEqual(a, b)
}
