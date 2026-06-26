package webclient

import "net/http"

// browserUserAgent is the User-Agent advertised by the disguise-aware backends.
// It is pinned to the same Chrome major version as tlsClientProfile so the tls
// backend never advertises a browser its TLS/HTTP2 fingerprint contradicts.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

// browserDefaultHeaders are the navigation headers a real Chrome sends. Backends
// add them — without overriding caller-supplied values — so a request reads as a
// browser navigation rather than a bare client.
var browserDefaultHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"Accept-Language": "en-US,en;q=0.9",
}

// ensureBrowserHeaders fills in the browser default headers and User-Agent on h
// for any entries the caller did not already set. It mutates and returns h,
// allocating a fresh header map when h is nil.
func ensureBrowserHeaders(h http.Header) http.Header {
	if h == nil {
		h = http.Header{}
	}
	for key, value := range browserDefaultHeaders {
		if h.Get(key) == "" {
			h.Set(key, value)
		}
	}
	if h.Get("User-Agent") == "" {
		h.Set("User-Agent", browserUserAgent)
	}
	return h
}
