package attacksurface

import "testing"

func TestSeverityForFeature(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"csp_missing", "high"},
		{"has_file_upload", "high"},
		{"csp_unsafe_inline", "medium"},
		{"hsts_missing", "medium"},
		{"num_params", "low"},
		{"unknown_feature", "low"},
	}

	for _, tc := range cases {
		if got := SeverityForFeature(tc.name); got != tc.want {
			t.Errorf("SeverityForFeature(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDescribeFeature_KnownAndFallback(t *testing.T) {
	if got := DescribeFeature("csp_missing"); got == "csp_missing" {
		t.Errorf("DescribeFeature(csp_missing) = %q, want human-readable description", got)
	}

	if got := DescribeFeature("num_scripts"); got != "Number of script tags on the page" {
		t.Errorf("DescribeFeature(num_scripts) = %q, want %q", got, "Number of script tags on the page")
	}

	name := "custom_feature"
	if got := DescribeFeature(name); got != name {
		t.Errorf("DescribeFeature(%q) = %q, want fallback to feature name", name, got)
	}
}
