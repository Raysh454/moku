package filter

import (
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check that security defaults are set
	if len(config.SkipExtensions) == 0 {
		t.Error("DefaultConfig() should have SkipExtensions")
	}

	// Verify security-important extensions are NOT in skip list
	securityExtensions := []string{".js", ".json", ".xml", ".html", ".php", ".svg", ".env", ".config"}
	for _, ext := range securityExtensions {
		for _, skip := range config.SkipExtensions {
			if skip == ext {
				t.Errorf("DefaultConfig() should not skip security-relevant extension %s", ext)
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
			t.Errorf("DefaultConfig() should skip binary extension %s", ext)
		}
	}

	// Status codes should be empty by default (404 is opt-in)
	if len(config.SkipStatusCodes) != 0 {
		t.Errorf("DefaultConfig() should not skip any status codes by default, got %v", config.SkipStatusCodes)
	}
}

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name   string
		global *Config
		site   *Config
		api    *Config
		want   *Config
	}{
		{
			name:   "all nil",
			global: nil,
			site:   nil,
			api:    nil,
			want:   &Config{},
		},
		{
			name: "only global",
			global: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
			site: nil,
			api:  nil,
			want: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
		},
		{
			name: "global + site merge",
			global: &Config{
				SkipExtensions: []string{".jpg"},
			},
			site: &Config{
				SkipExtensions: []string{".png"},
			},
			api: nil,
			want: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
		},
		{
			name: "all three levels merge",
			global: &Config{
				SkipExtensions: []string{".jpg"},
			},
			site: &Config{
				SkipPatterns: []string{"*/media/*"},
			},
			api: &Config{
				SkipStatusCodes: []int{404},
			},
			want: &Config{
				SkipExtensions:  []string{".jpg"},
				SkipPatterns:    []string{"*/media/*"},
				SkipStatusCodes: []int{404},
			},
		},
		{
			name: "deduplication",
			global: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
			site: &Config{
				SkipExtensions: []string{".png", ".gif"}, // .png is duplicate
			},
			api: nil,
			want: &Config{
				SkipExtensions: []string{".jpg", ".png", ".gif"},
			},
		},
		{
			name: "status codes merge",
			global: &Config{
				SkipStatusCodes: []int{404},
			},
			site: &Config{
				SkipStatusCodes: []int{410},
			},
			api: &Config{
				SkipStatusCodes: []int{500},
			},
			want: &Config{
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
	rules := []Rule{
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
	config := RulesToConfig([]Rule{})
	if !config.IsEmpty() {
		t.Error("RulesToConfig([]) should return empty config")
	}
}

func TestRulesToConfig_InvalidStatusCode(t *testing.T) {
	rules := []Rule{
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
