package filter

import (
	"testing"
)

func TestRuleType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		ruleType RuleType
		want     bool
	}{
		{"extension valid", RuleTypeExtension, true},
		{"pattern valid", RuleTypePattern, true},
		{"status_code valid", RuleTypeStatusCode, true},
		{"empty invalid", RuleType(""), false},
		{"unknown invalid", RuleType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ruleType.IsValid()
			if got != tt.want {
				t.Errorf("RuleType(%q).IsValid() = %v, want %v", tt.ruleType, got, tt.want)
			}
		})
	}
}

func TestFilterRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    FilterRule
		wantErr bool
	}{
		{
			name: "valid extension rule",
			rule: FilterRule{
				ID:        "rule-1",
				WebsiteID: "site-1",
				RuleType:  RuleTypeExtension,
				RuleValue: ".jpg",
			},
			wantErr: false,
		},
		{
			name: "valid pattern rule",
			rule: FilterRule{
				ID:        "rule-2",
				WebsiteID: "site-1",
				RuleType:  RuleTypePattern,
				RuleValue: "*/media/*",
			},
			wantErr: false,
		},
		{
			name: "valid status code rule",
			rule: FilterRule{
				ID:        "rule-3",
				WebsiteID: "site-1",
				RuleType:  RuleTypeStatusCode,
				RuleValue: "404",
			},
			wantErr: false,
		},
		{
			name: "missing rule type",
			rule: FilterRule{
				ID:        "rule-4",
				WebsiteID: "site-1",
				RuleValue: ".jpg",
			},
			wantErr: true,
		},
		{
			name: "missing rule value",
			rule: FilterRule{
				ID:        "rule-5",
				WebsiteID: "site-1",
				RuleType:  RuleTypeExtension,
			},
			wantErr: true,
		},
		{
			name: "invalid rule type",
			rule: FilterRule{
				ID:        "rule-6",
				WebsiteID: "site-1",
				RuleType:  RuleType("invalid"),
				RuleValue: ".jpg",
			},
			wantErr: true,
		},
		{
			name: "invalid status code - not a number",
			rule: FilterRule{
				ID:        "rule-7",
				WebsiteID: "site-1",
				RuleType:  RuleTypeStatusCode,
				RuleValue: "abc",
			},
			wantErr: true,
		},
		{
			name: "invalid status code - out of range",
			rule: FilterRule{
				ID:        "rule-8",
				WebsiteID: "site-1",
				RuleType:  RuleTypeStatusCode,
				RuleValue: "999",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterRule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateExtension(t *testing.T) {
	tests := []struct {
		name    string
		ext     string
		wantErr bool
	}{
		{"with dot", ".jpg", false},
		{"without dot", "jpg", true}, // requires dot
		{"uppercase", ".JPG", false},
		{"empty", "", true},
		{"just dot", ".", true},
		{"multiple dots", ".tar.gz", true}, // only alphanumeric allowed after dot
		{"with slash", ".js/", true},
		{"with spaces", ". js", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtension(tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtension(%q) error = %v, wantErr %v", tt.ext, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"simple glob", "*/media/*", false},
		{"double star", "**/assets/**", false},
		{"question mark", "file?.txt", false},
		{"literal path", "/static/", true}, // requires wildcard
		{"empty", "", true},
		{"complex pattern", "*/vendor/**/*.min.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestValidateStatusCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"200", "200", false},
		{"404", "404", false},
		{"500", "500", false},
		{"100", "100", false},
		{"599", "599", false},
		{"99 - too low", "99", true},
		{"600 - too high", "600", true},
		{"not a number", "abc", true},
		{"empty", "", true},
		{"negative", "-200", true},
		{"decimal", "200.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatusCode(%q) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
		})
	}
}

func TestFilterConfig_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		config FilterConfig
		want   bool
	}{
		{
			name:   "empty config",
			config: FilterConfig{},
			want:   true,
		},
		{
			name: "with skip extensions",
			config: FilterConfig{
				SkipExtensions: []string{".jpg"},
			},
			want: false,
		},
		{
			name: "with skip patterns",
			config: FilterConfig{
				SkipPatterns: []string{"*/media/*"},
			},
			want: false,
		},
		{
			name: "with skip status codes",
			config: FilterConfig{
				SkipStatusCodes: []int{404},
			},
			want: false,
		},
		{
			name: "with empty slices",
			config: FilterConfig{
				SkipExtensions:  []string{},
				SkipPatterns:    []string{},
				SkipStatusCodes: []int{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsEmpty()
			if got != tt.want {
				t.Errorf("FilterConfig.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
