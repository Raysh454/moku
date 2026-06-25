package webclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"github.com/raysh454/moku/internal/logging"
)

// tlsClientProfile is the Chrome fingerprint the backend impersonates. The
// User-Agent below MUST stay on the same Chrome major version: Cloudflare flags
// a request whose advertised browser disagrees with its TLS/HTTP2 fingerprint,
// so the two are intentionally pinned together.
var tlsClientProfile = profiles.Chrome_146

// tlsClientUserAgent matches tlsClientProfile's Chrome version.
const tlsClientUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

// browserDefaultHeaders are sent (unless the caller overrides them) so the
// request looks like a browser navigation rather than a bare client.
var browserDefaultHeaders = map[string]string{
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"Accept-Language": "en-US,en;q=0.9",
}

// TLSClientClient is a WebClient backed by bogdanfinn/tls-client. It impersonates
// a real Chrome TLS (JA3/JA4) and HTTP/2 fingerprint, which clears Cloudflare's
// transport-layer gate that bare net/http trips — without running a browser. It
// does not execute JavaScript, so it cannot pass an active JS/Turnstile
// challenge; escalate challenged targets to a browser backend.
//
// tls-client is not an *http.Transport, so the SSRF dial guard, redirect cap,
// and body-size cap that the nethttp backend gets from a guarded *http.Client
// are re-applied here: the guard via WithDialer(guardDialControl), the redirect
// cap via WithCustomRedirectFunc, and the body cap via an io.LimitReader.
type TLSClientClient struct {
	client       tlsclient.HttpClient
	userAgent    string
	maxBodyBytes int64
	logger       logging.Logger
}

// NewTLSClient constructs a tls-client-backed WebClient. When
// cfg.AllowPrivateHosts is false (the default) every dial is routed through the
// shared guardDialControl so private/loopback destinations are refused at the
// socket layer, matching the nethttp backend's SSRF posture.
func NewTLSClient(cfg Config, logger logging.Logger) (WebClient, error) {
	componentLogger := logger.With(logging.Field{Key: "backend", Value: "tls"})

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}

	options := []tlsclient.HttpClientOption{
		tlsclient.WithClientProfile(tlsClientProfile),
		tlsclient.WithTimeoutSeconds(int(dialGuardTimeout / time.Second)),
		tlsclient.WithCustomRedirectFunc(capRedirects),
	}
	if !cfg.AllowPrivateHosts {
		options = append(options, tlsclient.WithDialer(net.Dialer{
			Timeout: dialGuardTimeout,
			Control: guardDialControl,
		}))
	}

	inner, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("constructing tls-client: %w", err)
	}

	componentLogger.Info("created tls webclient",
		logging.Field{Key: "profile", Value: tlsClientProfile.GetClientHelloStr()},
		logging.Field{Key: "max_body_bytes", Value: maxBodyBytes})

	return &TLSClientClient{
		client:       inner,
		userAgent:    tlsClientUserAgent,
		maxBodyBytes: maxBodyBytes,
		logger:       componentLogger,
	}, nil
}

// capRedirects bounds the redirect chain at maxRedirectHops, mirroring the
// nethttp backend's CheckRedirect.
func capRedirects(_ *fhttp.Request, via []*fhttp.Request) error {
	if len(via) >= maxRedirectHops {
		return fmt.Errorf("stopped after %d redirects", maxRedirectHops)
	}
	return nil
}

// Do executes req through the impersonating client and returns a canonical
// Response. Only GET-style retrieval is expected, but any method with a body is
// supported for symmetry with the nethttp backend.
func (c *TLSClientClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := fhttp.NewRequestWithContext(ctx, method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.applyHeaders(httpReq, req.Headers)

	c.logger.Debug("tls request",
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "url", Value: req.URL})

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tls do: %w", err)
	}
	defer resp.Body.Close()

	// Read at most maxBodyBytes+1 so an over-cap body is detectable, then reject
	// it outright rather than truncate (a partial body corrupts snapshots/diffs).
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > c.maxBodyBytes {
		return nil, fmt.Errorf("%w: %s", ErrBodyTooLarge, req.URL)
	}

	return &Response{
		Request:    req,
		Body:       body,
		Headers:    convertHeader(resp.Header),
		StatusCode: resp.StatusCode,
		FetchedAt:  time.Now(),
	}, nil
}

// Get is a convenience method for simple GET requests.
func (c *TLSClientClient) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodGet, URL: url})
}

// Close releases the client's idle connections.
func (c *TLSClientClient) Close() error {
	c.logger.Info("closing tls webclient")
	c.client.CloseIdleConnections()
	return nil
}

// applyHeaders copies caller headers onto the request, fills in browser-default
// headers the caller did not set, and guarantees a User-Agent consistent with
// the impersonated TLS profile.
func (c *TLSClientClient) applyHeaders(req *fhttp.Request, headers http.Header) {
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	for key, value := range browserDefaultHeaders {
		if req.Header.Get(key) == "" {
			req.Header.Set(key, value)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
}

// convertHeader copies an fhttp.Header into a canonical net/http.Header so the
// rest of moku sees the same Response shape regardless of backend.
func convertHeader(src fhttp.Header) http.Header {
	dst := http.Header{}
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	return dst
}
