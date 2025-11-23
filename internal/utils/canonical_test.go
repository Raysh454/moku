package utils

import (
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		in   string
		opts CanonicalizeOptions
		want string
	}{
		{
			in:   "HTTP://Example.COM:80/foo/../bar/?b=2&a=1#frag",
			opts: CanonicalizeOptions{DefaultScheme: "", StripTrailingSlash: false},
			want: "http://example.com/bar?a=1&b=2",
		},
		{
			in:   "https://example.com:443/index.html#section",
			opts: CanonicalizeOptions{},
			want: "https://example.com/index.html",
		},
		{
			in:   "example.com/page?utm_source=x&utm_medium=y&z=1",
			opts: CanonicalizeOptions{DefaultScheme: "https", DropTrackingParams: true},
			want: "https://example.com/page?z=1",
		},
		{
			in:   "https://例え.テスト/a",
			opts: CanonicalizeOptions{},
			// punycode-encoded host
			want: "https://xn--r8jz45g.xn--zckzah/a",
		},
		{
			in:   "https://example.com/foo/",
			opts: CanonicalizeOptions{StripTrailingSlash: true},
			want: "https://example.com/foo",
		},
	}

	for _, tt := range tests {
		got, err := Canonicalize(tt.in, tt.opts)
		if err != nil {
			t.Fatalf("canonicalize(%q) error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("canonicalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
