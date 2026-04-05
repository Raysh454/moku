package filter

import (
	"testing"
)

func TestMatchExtension(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		extensions []string
		want       bool
	}{
		{"match jpg", "https://example.com/image.jpg", []string{".jpg", ".png"}, true},
		{"match png", "https://example.com/image.png", []string{".jpg", ".png"}, true},
		{"no match", "https://example.com/script.js", []string{".jpg", ".png"}, false},
		{"case insensitive", "https://example.com/IMAGE.JPG", []string{".jpg"}, true},
		{"ext without dot", "https://example.com/image.jpg", []string{"jpg"}, true},
		{"empty extensions", "https://example.com/image.jpg", []string{}, false},
		{"no extension in url", "https://example.com/path/file", []string{".jpg"}, false},
		{"query string", "https://example.com/image.jpg?v=1", []string{".jpg"}, true},
		{"fragment", "https://example.com/image.jpg#section", []string{".jpg"}, true},
		{"double extension", "https://example.com/file.tar.gz", []string{".gz"}, true},
		{"hidden file", "https://example.com/.htaccess", []string{".htaccess"}, true},
		{"path no extension", "https://example.com/api/users", []string{".html"}, false},
		{"trailing slash", "https://example.com/folder/", []string{".html"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := matchExtension(tt.url, tt.extensions)
			if got != tt.want {
				t.Errorf("matchExtension(%q, %v) = %v, want %v", tt.url, tt.extensions, got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		patterns []string
		want     bool
	}{
		{"simple wildcard", "https://example.com/media/video.mp4", []string{"*/media/*"}, true},
		{"double star", "https://example.com/a/b/c/file.js", []string{"**/file.js"}, true},
		{"no match", "https://example.com/api/users", []string{"*/media/*"}, false},
		{"question mark", "https://example.com/file1.txt", []string{"*/file?.txt"}, true},
		{"empty patterns", "https://example.com/media/video.mp4", []string{}, false},
		{"exact path", "https://example.com/static/", []string{"*/static/*"}, true},
		{"multiple patterns", "https://example.com/assets/style.css", []string{"*/media/*", "*/assets/*"}, true},
		{"complex pattern", "https://example.com/vendor/jquery/jquery.min.js", []string{"*/vendor/**/*.min.js"}, true},
		{"path segment", "https://example.com/api/v1/users", []string{"*/api/*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := matchPattern(tt.url, tt.patterns)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %v) = %v, want %v", tt.url, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		skipCodes  []int
		want       bool
	}{
		{"match 404", 404, []int{404, 410}, true},
		{"match 410", 410, []int{404, 410}, true},
		{"no match", 200, []int{404, 410}, false},
		{"empty codes", 404, []int{}, false},
		{"single code", 500, []int{500}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchStatusCode(tt.statusCode, tt.skipCodes)
			if got != tt.want {
				t.Errorf("matchStatusCode(%d, %v) = %v, want %v", tt.statusCode, tt.skipCodes, got, tt.want)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"simple wildcard", "*.txt", "file.txt", true},
		{"wildcard in middle", "file*.txt", "file123.txt", true},
		{"double star", "**/*.js", "a/b/c/file.js", true},
		{"question mark", "file?.txt", "file1.txt", true},
		{"question mark no match", "file?.txt", "file12.txt", false},
		{"literal match", "file.txt", "file.txt", true},
		{"no match", "*.txt", "file.js", false},
		{"path separator", "a/*/c", "a/b/c", true},
		{"double star middle", "a/**/c", "a/b/x/y/c", true},
		{"empty pattern", "", "", true},
		{"empty path", "*.txt", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
