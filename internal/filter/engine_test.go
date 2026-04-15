package filter

import (
	"testing"
)

func TestEngine_ShouldFilter(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		url    string
		want   bool
		reason string
	}{
		{
			name: "filter by extension",
			config: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
			url:    "https://example.com/image.jpg",
			want:   true,
			reason: "extension:.jpg",
		},
		{
			name: "filter by pattern",
			config: &Config{
				SkipPatterns: []string{"*/media/*"},
			},
			url:    "https://example.com/media/video.mp4",
			want:   true,
			reason: "pattern:*/media/*",
		},
		{
			name: "no filter - different extension",
			config: &Config{
				SkipExtensions: []string{".jpg", ".png"},
			},
			url:    "https://example.com/script.js",
			want:   false,
			reason: "",
		},
		{
			name:   "nil config",
			config: nil,
			url:    "https://example.com/image.jpg",
			want:   false,
			reason: "",
		},
		{
			name:   "empty config",
			config: &Config{},
			url:    "https://example.com/image.jpg",
			want:   false,
			reason: "",
		},
		{
			name: "combined filters - extension match",
			config: &Config{
				SkipExtensions: []string{".jpg"},
				SkipPatterns:   []string{"*/static/*"},
			},
			url:    "https://example.com/image.jpg",
			want:   true,
			reason: "extension:.jpg",
		},
		{
			name: "combined filters - pattern match",
			config: &Config{
				SkipExtensions: []string{".jpg"},
				SkipPatterns:   []string{"*/static/*"},
			},
			url:    "https://example.com/static/app.js",
			want:   true,
			reason: "pattern:*/static/*",
		},
		{
			name: "case insensitive extension",
			config: &Config{
				SkipExtensions: []string{".jpg"},
			},
			url:    "https://example.com/IMAGE.JPG",
			want:   true,
			reason: "extension:.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(tt.config)
			result := engine.ShouldFilter(tt.url)
			if result.Filtered != tt.want {
				t.Errorf("Engine.ShouldFilter(%q).Filtered = %v, want %v", tt.url, result.Filtered, tt.want)
			}
			if result.Reason != tt.reason {
				t.Errorf("Engine.ShouldFilter(%q).Reason = %q, want %q", tt.url, result.Reason, tt.reason)
			}
		})
	}
}

func TestEngine_ShouldFilterStatus(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		code   int
		want   bool
		reason string
	}{
		{
			name: "filter 404",
			config: &Config{
				SkipStatusCodes: []int{404, 410},
			},
			code:   404,
			want:   true,
			reason: "status_code:404",
		},
		{
			name: "filter 410",
			config: &Config{
				SkipStatusCodes: []int{404, 410},
			},
			code:   410,
			want:   true,
			reason: "status_code:410",
		},
		{
			name: "no filter - 200",
			config: &Config{
				SkipStatusCodes: []int{404, 410},
			},
			code:   200,
			want:   false,
			reason: "",
		},
		{
			name:   "nil config",
			config: nil,
			code:   404,
			want:   false,
			reason: "",
		},
		{
			name:   "empty config",
			config: &Config{},
			code:   404,
			want:   false,
			reason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(tt.config)
			result := engine.ShouldFilterStatus(tt.code)
			if result.Filtered != tt.want {
				t.Errorf("Engine.ShouldFilterStatus(%d).Filtered = %v, want %v", tt.code, result.Filtered, tt.want)
			}
			if result.Reason != tt.reason {
				t.Errorf("Engine.ShouldFilterStatus(%d).Reason = %q, want %q", tt.code, result.Reason, tt.reason)
			}
		})
	}
}

func TestNewEngine_NilConfig(t *testing.T) {
	engine := NewEngine(nil)
	if engine == nil {
		t.Error("NewEngine(nil) should return a valid engine")
	}

	// Should not panic and should not filter anything
	result := engine.ShouldFilter("https://example.com/image.jpg")
	if result.Filtered {
		t.Error("Engine with nil config should not filter anything")
	}
}
