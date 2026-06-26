package webclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/raysh454/moku/internal/logging"
)

// remoteSolveTimeout is the per-request budget handed to the unblocker (the
// maxTimeout field), in line with the time a browser-backed solve can take.
const remoteSolveTimeout = 60 * time.Second

// remoteRequestTimeout bounds the whole HTTP call to the unblocker; it sits
// above remoteSolveTimeout so the solver, not the transport, decides the budget.
const remoteRequestTimeout = 90 * time.Second

// RemoteClient is a WebClient that delegates fetching to an out-of-process
// unblocker exposing the FlareSolverr/Byparr /v1 protocol (a self-hostable
// browser-backed solver). It is the heaviest, most capable tier: it executes
// JavaScript and can clear challenges the in-process backends cannot, at the
// cost of running a separate service. moku only issues an HTTP call and maps the
// returned solution onto the canonical Response.
type RemoteClient struct {
	endpoint     string
	httpClient   *http.Client
	maxBodyBytes int64
	solveTimeout time.Duration
	logger       logging.Logger
}

// NewRemoteClient builds a RemoteClient. endpoint is the full unblocker URL
// (e.g. http://127.0.0.1:8191/v1). The endpoint is trusted operator
// configuration, so the SSRF dial guard is intentionally not applied to it — the
// unblocker, not moku, dials the target.
func NewRemoteClient(endpoint string, cfg Config, logger logging.Logger) (WebClient, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("remote client requires an endpoint URL")
	}
	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	return &RemoteClient{
		endpoint:     endpoint,
		httpClient:   &http.Client{Timeout: remoteRequestTimeout},
		maxBodyBytes: maxBodyBytes,
		solveTimeout: remoteSolveTimeout,
		logger:       logger.With(logging.Field{Key: "backend", Value: "remote"}),
	}, nil
}

// solverRequest is the FlareSolverr/Byparr /v1 request envelope.
type solverRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	MaxTimeout int64  `json:"maxTimeout"`
}

// solverResponse is the subset of the /v1 reply moku consumes.
type solverResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution struct {
		URL      string            `json:"url"`
		Status   int               `json:"status"`
		Headers  map[string]string `json:"headers"`
		Response string            `json:"response"`
	} `json:"solution"`
}

// Do submits a GET solve to the unblocker and maps the solution onto a Response.
func (c *RemoteClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet {
		return nil, fmt.Errorf("remote client: method %q not supported", method)
	}

	payload, err := json.Marshal(solverRequest{
		Cmd:        "request.get",
		URL:        req.URL,
		MaxTimeout: c.solveTimeout.Milliseconds(),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding solver request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Debug("remote solve", logging.Field{Key: "url", Value: req.URL})

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("remote do: %w", err)
	}
	defer httpResp.Body.Close()

	// The solver wraps the page HTML in JSON, so allow generous headroom over the
	// body cap for the envelope and escaping; the cap is enforced on the decoded
	// HTML below, not on the wire bytes.
	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, c.maxBodyBytes*2+(1<<16)))
	if err != nil {
		return nil, fmt.Errorf("read solver response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote unblocker returned status %d", httpResp.StatusCode)
	}

	var solved solverResponse
	if err := json.Unmarshal(raw, &solved); err != nil {
		return nil, fmt.Errorf("decoding solver response: %w", err)
	}
	if !strings.EqualFold(solved.Status, "ok") {
		return nil, fmt.Errorf("remote unblocker did not solve %s: %s", req.URL, solved.Message)
	}

	body := []byte(solved.Solution.Response)
	if int64(len(body)) > c.maxBodyBytes {
		return nil, fmt.Errorf("%w: %s", ErrBodyTooLarge, req.URL)
	}

	return &Response{
		Request:    req,
		Body:       body,
		Headers:    solverHeaders(solved.Solution.Headers),
		StatusCode: solved.Solution.Status,
		FetchedAt:  time.Now(),
	}, nil
}

// Get is a convenience method for simple GET requests.
func (c *RemoteClient) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, &Request{Method: http.MethodGet, URL: url})
}

// Close releases idle connections to the unblocker.
func (c *RemoteClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// solverHeaders converts the unblocker's single-valued header map into the
// canonical net/http.Header.
func solverHeaders(src map[string]string) http.Header {
	h := http.Header{}
	for key, value := range src {
		h.Set(key, value)
	}
	return h
}
