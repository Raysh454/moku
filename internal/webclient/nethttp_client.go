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
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

//net/http backed implementation of webclient.
type NetHTTPClient struct {
	client *http.Client
	logger interfaces.Logger
}

func NewNetHTTPClient(cfg *app.Config, logger interfaces.Logger) (interfaces.WebClient, error) {
	// Create component-scoped logger
	componentLogger := logger.With(interfaces.Field{Key: "backend", Value: "nethttp"})
	
	// Use default timeout if cfg doesn't specify otherwise
	client := &http.Client{Timeout: 30 * time.Second}
	
	componentLogger.Info("created nethttp webclient",
		interfaces.Field{Key: "timeout", Value: client.Timeout.String()})
	
	return &NetHTTPClient{
		client: client,
		logger: componentLogger,
	}, nil
}

// Do implements the generic request execution using net/http.
func (nhc *NetHTTPClient) Do(ctx context.Context, req *model.Request) (*model.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(req.Method)

	nhc.logger.Debug("sending http request",
		interfaces.Field{Key: "method", Value: method},
		interfaces.Field{Key: "url", Value: req.URL})

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
			interfaces.Field{Key: "method", Value: method},
			interfaces.Field{Key: "url", Value: req.URL},
			interfaces.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("http do: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		nhc.logger.Warn("failed to read response body",
			interfaces.Field{Key: "method", Value: method},
			interfaces.Field{Key: "url", Value: req.URL},
			interfaces.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &model.Response{
		Request: req,
		Body: body,
		Headers: resp.Header,
		StatusCode: resp.StatusCode,
		FetchedAt: time.Now(),
	}, nil
}

// Get is a convenience method for simple GET requests
func (nhc *NetHTTPClient) Get(ctx context.Context, url string) (*model.Response, error) {
	req := &model.Request{
		Method: "GET",
		URL:    url,
	}
	return nhc.Do(ctx, req)
}

func (nhc *NetHTTPClient) Close() error {
	nhc.logger.Info("closing nethttp webclient")
	return nil
}
