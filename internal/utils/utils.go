package utils

import (
	"fmt"
	"net/url"
	"strings"
)

type URLTools struct {
	URL *url.URL
}

func NewURLTools(raw string) (*URLTools, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse url %s: %w", raw, err)
	}

	urlTools := &URLTools{
		URL: u,
	}
	urlTools.normalize()

	return urlTools, nil
}

func (u *URLTools) normalize() {
	u.URL.Fragment = ""
	u.URL.Scheme = strings.ToLower(u.URL.Scheme)
	u.URL.Host = strings.ToLower(u.URL.Host)

	if (u.URL.Scheme == "http" && strings.HasSuffix(u.URL.Host, ":80")) ||
		(u.URL.Scheme == "https" && strings.HasSuffix(u.URL.Host, ":443")) {
		u.URL.Host, _, _ = strings.Cut(u.URL.Host, ":")
	}

	u.URL.Path = strings.TrimRight(u.URL.Path, "/")
}

func (u *URLTools) DomainIsSame(target *URLTools) bool {
	return u.URL.Hostname() == target.URL.Hostname()
}

func (u *URLTools) DomainIsSameString(targetURL string) (bool, error) {
	parsed, err := NewURLTools(targetURL)
	if err != nil {
		return false, err
	}

	return u.URL.Hostname() == parsed.URL.Hostname(), nil
}

// ResolveFullUrlString resolves targetURL against u.URL and returns a full absolute URL.
//
// Examples:
//
//	Base: https://example.com/app/
//	ResolveFullUrlString("users")        → "https://example.com/app/users/"
//	ResolveFullUrlString("../login")     → "https://example.com/login/"
//	ResolveFullUrlString("/static")      → "https://example.com/static/"
//	ResolveFullUrlString("https://foo.com/x")
//	                                    → "https://foo.com/x/"
//
// Note: A trailing slash is always added to targetURL before resolution.
func (u *URLTools) ResolveFullUrlString(targetURL string) (string, error) {
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	parsed, err := NewURLTools(targetURL)
	if err != nil {
		return "", err
	}

	return u.URL.ResolveReference(parsed.URL).String(), nil
}

// GetPath returns the URL path without a trailing slash (except root).
//
// Examples:
//
//	URL: https://example.com/api/v1/    → GetPath() = "/api/v1"
//	URL: https://example.com/users      → GetPath() = "/users"
//	URL: https://example.com/           → GetPath() = ""
//	URL: https://example.com            → GetPath() = ""
func (u *URLTools) GetPath() string {
	if path := u.URL.Path; strings.HasSuffix(path, "/") {
		return path[:len(path)-1]
	} else {
		return path
	}
}
