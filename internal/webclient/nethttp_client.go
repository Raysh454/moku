package webclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/model"
)

type NetHTTPClient struct {
	client *http.Client
}

func NewNetHTTPClient(client *http.Client) *NetHTTPClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	
	return &NetHTTPClient{client: client}
}

// Do implements the generic request execution using net/http.
func (nhc *NetHTTPClient) Do(ctx context.Context, req *model.Request) (res *model.Response, err error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	method := strings.ToUpper(req.Method)

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
		return nil, fmt.Errorf("http do: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
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


