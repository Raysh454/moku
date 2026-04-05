package filter

import (
	"testing"
)

func TestEngine_ShouldFilter(t *testing.T) {
	tests := []struct {
		name   string
		config *FilterConfig
		url    string
		want   bool
		reason string
	}{
		{
			name: "filter by extension",
			config: &FilterConfig{
				SkipExtensions: []string{".jpg", ".png"},
			},
			url:    "https://example.com/image.jpg",
			want:   true,
			reason: "extension:.jpg",
		},
		{
			name: "filter by pattern",
			config: &FilterConfig{
				SkipPatterns: []string{"*/media/*"},
			},
			url:    "https://example.com/media/video.mp4",
			want:   true,
			reason: "pattern:*/media/*",
		},
		{
			name: "no filter - different extension",
			config: &FilterConfig{
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
			config: &FilterConfig{},
			url:    "https://example.com/image.jpg",
			want:   false,
			reason: "",
		},
		{
			name: "combined filters - extension match",
			config: &FilterConfig{
				SkipExtensions: []string{".jpg"},
				SkipPatterns:   []string{"*/static/*"},
			},
			url:    "https://example.com/image.jpg",
			want:   true,
			reason: "extension:.jpg",
		},
		{
			name: "combined filters - pattern match",
			config: &FilterConfig{
				SkipExtensions: []string{".jpg"},
				SkipPatterns:   []string{"*/static/*"},
			},
			url:    "https://example.com/static/app.js",
			want:   true,
			reason: "pattern:*/static/*",
		},
		{
			name: "case insensitive extension",
			config: &FilterConfig{
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
		config *FilterConfig
		code   int
		want   bool
		reason string
	}{
		{
			name: "filter 404",
			config: &FilterConfig{
				SkipStatusCodes: []int{404, 410},
			},
			code:   404,
			want:   true,
			reason: "status_code:404",
		},
		{
			name: "filter 410",
			config: &FilterConfig{
				SkipStatusCodes: []int{404, 410},
			},
			code:   410,
			want:   true,
			reason: "status_code:410",
		},
		{
			name: "no filter - 200",
			config: &FilterConfig{
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
			config: &FilterConfig{},
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

func TestEngine_FilterURLs(t *testing.T) {
	config := &FilterConfig{
		SkipExtensions: []string{".jpg", ".png", ".gif"},
		SkipPatterns:   []string{"*/media/*"},
	}
	engine := NewEngine(config)

	urls := []string{
		"https://example.com/page.html",
		"https://example.com/image.jpg",
		"https://example.com/media/video.mp4",
		"https://example.com/api/users",
		"https://example.com/logo.png",
	}

	kept, filtered := engine.FilterURLs(urls)

	expectedFiltered := 3 // .jpg, /media/, .png
	expectedKept := 2     // .html, /api/

	if len(filtered) != expectedFiltered {
		t.Errorf("FilterURLs() filtered count = %d, want %d", len(filtered), expectedFiltered)
	}
	if len(kept) != expectedKept {
		t.Errorf("FilterURLs() kept count = %d, want %d", len(kept), expectedKept)
	}

	// Check specific URLs - kept should not include filtered extensions
	for _, url := range kept {
		if url == "https://example.com/image.jpg" || url == "https://example.com/logo.png" {
			t.Errorf("FilterURLs() should have filtered %s", url)
		}
	}

	// Check filtered URLs contain expected ones
	filteredURLs := make(map[string]bool)
	for _, f := range filtered {
		filteredURLs[f.URL] = true
	}
	if !filteredURLs["https://example.com/image.jpg"] {
		t.Error("FilterURLs() should have filtered image.jpg")
	}
	if !filteredURLs["https://example.com/logo.png"] {
		t.Error("FilterURLs() should have filtered logo.png")
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
