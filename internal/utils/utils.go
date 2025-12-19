package utils

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/webclient"
	"golang.org/x/net/idna"
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
//	URL: https://example.com/api/v1/    → GetPath() = "/api/v1"internal/index/index.go
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

// CanonicalizeOptions controls optional canonicalization policies.
type CanonicalizeOptions struct {
	DropTrackingParams     bool     // remove common tracking params (utm_*, gclid, fbclid, ...)
	StripTrailingSlash     bool     // treat /a and /a/ the same by removing trailing slash (except for root "/")
	DefaultScheme          string   // if empty, require scheme in input; otherwise assume this scheme for schemeless URLs
	TrackingParamAllowlist []string // optional allowlist for query params (if non-empty, only these survive)
}

// Common tracking params to strip when DropTrackingParams is true.
var defaultTrackingParams = map[string]struct{}{
	"utm_source": {}, "utm_medium": {}, "utm_campaign": {}, "utm_term": {}, "utm_content": {},
	"gclid": {}, "fbclid": {}, "mc_cid": {}, "mc_eid": {},
}

// Canonicalize returns a deterministic canonical URL string or an error.
// It uses net/url plus path.Clean and sorts query params for determinism.
func Canonicalize(raw string, opts CanonicalizeOptions) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", &url.Error{Op: "parse", URL: raw, Err: ErrEmptyURL}
	}

	// If user provided a default scheme and the input has none, prepend it.
	if opts.DefaultScheme != "" && !strings.Contains(raw, "://") {
		raw = opts.DefaultScheme + "://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	// Must have host
	if u.Host == "" {
		return "", &url.Error{Op: "parse", URL: raw, Err: ErrMissingHost}
	}

	// Lowercase scheme and hostname
	u.Scheme = strings.ToLower(u.Scheme)

	// Lowercase host and convert IDN -> punycode
	host := u.Hostname()
	host = strings.ToLower(host)
	puny, err := idna.Lookup.ToASCII(host)
	if err == nil {
		host = puny
	}

	// Preserve non-default port only
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		u.Host = host
	} else if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}

	// Drop userinfo (credentials)
	u.User = nil

	// Normalize path
	cleanPath := path.Clean(u.Path)
	if cleanPath == "." {
		cleanPath = "/"
	}
	// Preserve or strip trailing slash according to policy (keep root "/")
	if opts.StripTrailingSlash && len(cleanPath) > 1 {
		cleanPath = strings.TrimRight(cleanPath, "/")
		if cleanPath == "" {
			cleanPath = "/"
		}
	}
	u.Path = cleanPath

	// Remove fragment
	u.Fragment = ""

	// Normalize query: remove tracking params (optional), apply allowlist (optional), sort keys and values
	q := u.Query()
	// remove tracking params if requested
	if opts.DropTrackingParams {
		for k := range q {
			if isAllowedByAllowlist(k, opts.TrackingParamAllowlist) {
				continue
			}
			if _, ok := defaultTrackingParams[strings.ToLower(k)]; ok {
				q.Del(k)
			}
		}
	}
	// If an allowlist is specified, drop any param not in it
	if len(opts.TrackingParamAllowlist) > 0 {
		allow := map[string]struct{}{}
		for _, k := range opts.TrackingParamAllowlist {
			allow[k] = struct{}{}
		}
		for k := range q {
			if _, ok := allow[k]; !ok {
				q.Del(k)
			}
		}
	}

	// Sort keys and values for deterministic encoding
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := url.Values{}
	for _, k := range keys {
		values := q[k]
		sort.Strings(values)
		for _, v := range values {
			ordered.Add(k, v)
		}
	}
	u.RawQuery = ordered.Encode()

	// Return the canonical string. url.URL.String will re-escape as needed.
	return u.String(), nil
}

// helper: return true when key is explicitly allowed via allowlist.
func isAllowedByAllowlist(key string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return false
	}
	for _, a := range allowlist {
		if key == a {
			return true
		}
	}
	return false
}

// Errors
var (
	ErrEmptyURL    = &url.Error{Op: "canonicalize", URL: "", Err: &errStr{"empty url"}}
	ErrMissingHost = &url.Error{Op: "canonicalize", URL: "", Err: &errStr{"missing host"}}
)

type errStr struct{ s string }

func (e *errStr) Error() string { return e.s }

// NewSnapshotFromResponse converts a webclient.Response to a model.Snapshot.
func NewSnapshotFromResponse(resp *webclient.Response) *models.Snapshot {
	if resp == nil {
		return nil
	}

	for k, v := range resp.Headers {
		// Lowercase header keys for consistency
		lowerKey := strings.ToLower(k)
		if lowerKey != k {
			delete(resp.Headers, k)
			resp.Headers[lowerKey] = v
		}
	}

	snap := &models.Snapshot{
		// ID left empty; tracker will assign one when persisting
		StatusCode: resp.StatusCode,
		URL:        resp.Request.URL,
		Body:       resp.Body, // caller may reuse resp.Body; if you want a copy, copy bytes here
		Headers:    resp.Headers,
		CreatedAt:  resp.FetchedAt,
	}

	return snap
}
