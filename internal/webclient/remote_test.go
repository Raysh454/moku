package webclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

// flareSolverrStub mimics a FlareSolverr/Byparr /v1 endpoint. It records the
// decoded request and replies with a configurable raw body.
type flareSolverrStub struct {
	gotCmd string
	gotURL string
	reply  string
	status int // HTTP status of the endpoint response (0 -> 200)
}

func (s *flareSolverrStub) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Cmd string `json:"cmd"`
			URL string `json:"url"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		s.gotCmd = body.Cmd
		s.gotURL = body.URL
		if s.status != 0 {
			w.WriteHeader(s.status)
		}
		_, _ = io.WriteString(w, s.reply)
	}
}

// okSolution renders a FlareSolverr "ok" response wrapping the given HTML body.
func okSolution(t *testing.T, htmlBody string, status int) string {
	t.Helper()
	payload := map[string]any{
		"status":  "ok",
		"message": "",
		"solution": map[string]any{
			"url":      "https://t",
			"status":   status,
			"headers":  map[string]string{"content-type": "text/html"},
			"response": htmlBody,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal solution: %v", err)
	}
	return string(b)
}

func TestNewRemoteClient_EmptyEndpoint_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := webclient.NewRemoteClient("", webclient.Config{}, &noopLogger{}); err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestRemoteClient_Do_MapsSolutionToResponse(t *testing.T) {
	t.Parallel()
	stub := &flareSolverrStub{reply: ""}
	ts := httptest.NewServer(stub.handler())
	defer ts.Close()
	stub.reply = okSolution(t, "<html>unblocked</html>", 200)

	client, err := webclient.NewRemoteClient(ts.URL, webclient.Config{}, &noopLogger{})
	if err != nil {
		t.Fatalf("NewRemoteClient: %v", err)
	}
	defer client.Close()

	resp, err := client.Get(context.Background(), "https://target.example/path")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stub.gotCmd != "request.get" {
		t.Errorf("expected cmd 'request.get', got %q", stub.gotCmd)
	}
	if stub.gotURL != "https://target.example/path" {
		t.Errorf("expected target URL forwarded, got %q", stub.gotURL)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 from solution, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "<html>unblocked</html>" {
		t.Errorf("expected solution HTML as body, got %q", resp.Body)
	}
	if resp.Headers.Get("Content-Type") != "text/html" {
		t.Errorf("expected solution headers mapped, got %q", resp.Headers.Get("Content-Type"))
	}
}

func TestRemoteClient_Do_SolverError_ReturnsError(t *testing.T) {
	t.Parallel()
	stub := &flareSolverrStub{reply: `{"status":"error","message":"challenge not solved"}`}
	ts := httptest.NewServer(stub.handler())
	defer ts.Close()

	client, _ := webclient.NewRemoteClient(ts.URL, webclient.Config{}, &noopLogger{})
	defer client.Close()

	if _, err := client.Get(context.Background(), "https://t"); err == nil {
		t.Fatal("expected error when solver reports status != ok")
	}
}

func TestRemoteClient_Do_EndpointNon200_ReturnsError(t *testing.T) {
	t.Parallel()
	stub := &flareSolverrStub{reply: "gateway boom", status: http.StatusBadGateway}
	ts := httptest.NewServer(stub.handler())
	defer ts.Close()

	client, _ := webclient.NewRemoteClient(ts.URL, webclient.Config{}, &noopLogger{})
	defer client.Close()

	if _, err := client.Get(context.Background(), "https://t"); err == nil {
		t.Fatal("expected error when endpoint returns a non-200 status")
	}
}

func TestRemoteClient_Do_BodyCap_RejectsOversize(t *testing.T) {
	t.Parallel()
	stub := &flareSolverrStub{}
	ts := httptest.NewServer(stub.handler())
	defer ts.Close()
	stub.reply = okSolution(t, strings.Repeat("X", 100), 200)

	client, _ := webclient.NewRemoteClient(ts.URL, webclient.Config{MaxBodyBytes: 8}, &noopLogger{})
	defer client.Close()

	if _, err := client.Get(context.Background(), "https://t"); err == nil {
		t.Error("expected ErrBodyTooLarge for an oversize solution body")
	}
}

func TestRemoteClient_Do_NonGET_ReturnsError(t *testing.T) {
	t.Parallel()
	stub := &flareSolverrStub{}
	ts := httptest.NewServer(stub.handler())
	defer ts.Close()
	stub.reply = okSolution(t, "x", 200)

	client, _ := webclient.NewRemoteClient(ts.URL, webclient.Config{}, &noopLogger{})
	defer client.Close()

	if _, err := client.Do(context.Background(), &webclient.Request{Method: "POST", URL: "https://t"}); err == nil {
		t.Fatal("expected error for unsupported method")
	}
}
