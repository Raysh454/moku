package webclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/logging"
)

// net/http backed implementation of webclient.
type NetHTTPClient struct {
	client       *http.Client
	logger       logging.Logger
	maxBodyBytes int64
}

// NewNetHTTPClient constructs a net/http-backed WebClient.
//
// When httpClient is nil the constructor builds a guarded client via
// newGuardedHTTPClient(cfg): the SSRF dialer guard (unless cfg.AllowPrivateHosts
// is set) and the redirect cap are applied. An injected *http.Client bypasses
// the dial guard by design — it is the testing seam used to point the client at
// loopback httptest servers — but the response-body cap (cfg.MaxBodyBytes, or
// DefaultMaxBodyBytes when zero) always applies regardless of how the client
// was constructed.
func NewNetHTTPClient(cfg Config, logger logging.Logger, httpClient *http.Client) (WebClient, error) {
	// Create component-scoped logger
	componentLogger := logger.With(logging.Field{Key: "backend", Value: "nethttp"})

	// If httpClient is nil, construct a guarded default (SSRF + redirect caps).
	if httpClient == nil {
		httpClient = newGuardedHTTPClient(cfg)
	}

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}

	componentLogger.Info("created nethttp webclient",
		logging.Field{Key: "timeout", Value: httpClient.Timeout.String()},
		logging.Field{Key: "max_body_bytes", Value: maxBodyBytes})

	return &NetHTTPClient{
		client:       httpClient,
		logger:       componentLogger,
		maxBodyBytes: maxBodyBytes,
	}, nil
}

// Do implements the generic request execution using net/http.
func (nhc *NetHTTPClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(req.Method)

	nhc.logger.Debug("sending http request",
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "url", Value: req.URL})

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if req.Headers != nil {
		for k, vs := range req.Headers {
			for _, v := range vs {
				httpReq.Header.Add(k, v)
			}
		}
	}

	resp, err := nhc.client.Do(httpReq)
	if err != nil {
		nhc.logger.Warn("http request failed",
			logging.Field{Key: "method", Value: method},
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("http do: %w", err)
	}

	defer resp.Body.Close()
	// Read at most maxBodyBytes+1 so an over-cap body is detectable: if the
	// read yields more than maxBodyBytes, the response is too large and is
	// rejected outright (no truncation, which would corrupt snapshots/diffs).
	body, err := io.ReadAll(io.LimitReader(resp.Body, nhc.maxBodyBytes+1))
	if err != nil {
		nhc.logger.Warn("failed to read response body",
			logging.Field{Key: "method", Value: method},
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > nhc.maxBodyBytes {
		nhc.logger.Warn("response body exceeds maximum allowed size",
			logging.Field{Key: "method", Value: method},
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "max_body_bytes", Value: nhc.maxBodyBytes})
		return nil, fmt.Errorf("%w: %s", ErrBodyTooLarge, req.URL)
	}

	return &Response{
		Request:    req,
		Body:       body,
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		FetchedAt:  time.Now(),
	}, nil
}

// Get is a convenience method for simple GET requests
func (nhc *NetHTTPClient) Get(ctx context.Context, url string) (*Response, error) {
	req := &Request{
		Method: "GET",
		URL:    url,
	}
	return nhc.Do(ctx, req)
}

func (nhc *NetHTTPClient) Close() error {
	nhc.logger.Info("closing nethttp webclient")
	return nil
}

// HTTPClient returns the underlying *http.Client
func (nhc *NetHTTPClient) HTTPClient() *http.Client {
	return nhc.client
}

// DoHTTPRequest executes a raw *http.Request and returns the *http.Response
func (nhc *NetHTTPClient) DoHTTPRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, nhc.ErrInvalidRequest()
	}

	// Set context if not already set (only if context is nil or context.TODO)
	if req.Context() == nil || req.Context() == context.TODO() {
		req = req.WithContext(ctx)
	}

	return nhc.client.Do(req)
}

// ErrInvalidRequest returns an error for invalid request scenarios
func (nhc *NetHTTPClient) ErrInvalidRequest() error {
	return fmt.Errorf("request cannot be nil")
}
