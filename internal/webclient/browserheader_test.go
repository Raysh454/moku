package webclient_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

// recordingClient captures the request it was asked to perform so a decorator's
// effect on the request can be asserted.
type recordingClient struct {
	gotReq *webclient.Request
	resp   *webclient.Response
	err    error
	closed bool
}

func (r *recordingClient) Do(_ context.Context, req *webclient.Request) (*webclient.Response, error) {
	r.gotReq = req
	return r.resp, r.err
}

func (r *recordingClient) Get(ctx context.Context, url string) (*webclient.Response, error) {
	return r.Do(ctx, &webclient.Request{Method: "GET", URL: url})
}

func (r *recordingClient) Close() error {
	r.closed = true
	return nil
}

func TestBrowserHeaderClient_InjectsBrowserHeadersWhenAbsent(t *testing.T) {
	t.Parallel()
	inner := &recordingClient{resp: &webclient.Response{StatusCode: 200}}
	client := webclient.NewBrowserHeaderClient(inner, &noopLogger{})

	if _, err := client.Get(context.Background(), "https://x"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	got := inner.gotReq.Headers
	if got.Get("User-Agent") == "" {
		t.Error("expected a User-Agent to be injected")
	}
	if got.Get("Accept") == "" {
		t.Error("expected an Accept header to be injected")
	}
}

func TestBrowserHeaderClient_DoesNotOverrideCallerHeaders(t *testing.T) {
	t.Parallel()
	inner := &recordingClient{resp: &webclient.Response{StatusCode: 200}}
	client := webclient.NewBrowserHeaderClient(inner, &noopLogger{})

	hdr := http.Header{}
	hdr.Set("User-Agent", "caller/1.0")

	if _, err := client.Do(context.Background(), &webclient.Request{Method: "GET", URL: "https://x", Headers: hdr}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := inner.gotReq.Headers.Get("User-Agent"); got != "caller/1.0" {
		t.Errorf("expected caller User-Agent preserved, got %q", got)
	}
}

func TestBrowserHeaderClient_DoesNotMutateCallerHeaderMap(t *testing.T) {
	t.Parallel()
	inner := &recordingClient{resp: &webclient.Response{StatusCode: 200}}
	client := webclient.NewBrowserHeaderClient(inner, &noopLogger{})

	callerHdr := http.Header{}
	if _, err := client.Do(context.Background(), &webclient.Request{Method: "GET", URL: "https://x", Headers: callerHdr}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(callerHdr) != 0 {
		t.Errorf("decorator must not mutate the caller's header map, got %v", callerHdr)
	}
}

func TestBrowserHeaderClient_DelegatesAndCloses(t *testing.T) {
	t.Parallel()
	inner := &recordingClient{resp: &webclient.Response{StatusCode: 204, Body: []byte("x")}}
	client := webclient.NewBrowserHeaderClient(inner, &noopLogger{})

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("expected delegated status 204, got %d", resp.StatusCode)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !inner.closed {
		t.Error("expected Close to delegate to the inner client")
	}
}
