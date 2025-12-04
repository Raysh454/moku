package webclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
)

// net/http backed implementation of webclient.
type NetHTTPClient struct {
	client *http.Client
	logger logging.Logger
}

func NewNetHTTPClient(cfg *app.Config, logger logging.Logger, httpClient *http.Client) (WebClient, error) {
	// Create component-scoped logger
	componentLogger := logger.With(logging.Field{Key: "backend", Value: "nethttp"})

	// If httpClient is nil, construct a sensible default with timeout from cfg or fallback to 30s
	if httpClient == nil {
		// TODO: Consider reading timeout from cfg if added in the future
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	componentLogger.Info("created nethttp webclient",
		logging.Field{Key: "timeout", Value: httpClient.Timeout.String()})

	return &NetHTTPClient{
		client: httpClient,
		logger: componentLogger,
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		nhc.logger.Warn("failed to read response body",
			logging.Field{Key: "method", Value: method},
			logging.Field{Key: "url", Value: req.URL},
			logging.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("read body: %w", err)
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
