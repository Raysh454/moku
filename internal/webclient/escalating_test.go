package webclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

// scriptedClient is a WebClient test double returning a fixed response/error and
// recording how many times it was invoked and whether it was closed.
type scriptedClient struct {
	resp     *webclient.Response
	err      error
	calls    int
	closed   bool
	closeErr error
}

func (s *scriptedClient) Do(_ context.Context, _ *webclient.Request) (*webclient.Response, error) {
	s.calls++
	return s.resp, s.err
}

func (s *scriptedClient) Get(ctx context.Context, url string) (*webclient.Response, error) {
	return s.Do(ctx, &webclient.Request{Method: "GET", URL: url})
}

func (s *scriptedClient) Close() error {
	s.closed = true
	return s.closeErr
}

// escalateOn403 is a simple predicate: a blocked status means "try the next tier".
func escalateOn403(resp *webclient.Response) bool {
	return resp != nil && resp.StatusCode == 403
}

func okResp(status int) *webclient.Response {
	return &webclient.Response{StatusCode: status, Body: []byte("body")}
}

func TestNewEscalatingClient_NoTiers_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := webclient.NewEscalatingClient(nil, escalateOn403, &noopLogger{})
	if err == nil {
		t.Fatal("expected error when constructing with no tiers")
	}
}

func TestNewEscalatingClient_NilPredicate_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := webclient.NewEscalatingClient([]webclient.WebClient{&scriptedClient{}}, nil, &noopLogger{})
	if err == nil {
		t.Fatal("expected error when constructing with a nil predicate")
	}
}

func TestEscalatingClient_FirstTierSatisfies_DoesNotEscalate(t *testing.T) {
	t.Parallel()
	tier0 := &scriptedClient{resp: okResp(200)}
	tier1 := &scriptedClient{resp: okResp(200)}

	client, err := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})
	if err != nil {
		t.Fatalf("NewEscalatingClient: %v", err)
	}

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 from tier0, got %d", resp.StatusCode)
	}
	if tier0.calls != 1 {
		t.Errorf("expected tier0 called once, got %d", tier0.calls)
	}
	if tier1.calls != 0 {
		t.Errorf("expected tier1 not called when tier0 satisfies, got %d", tier1.calls)
	}
}

func TestEscalatingClient_FirstTierBlocked_EscalatesToNext(t *testing.T) {
	t.Parallel()
	tier0 := &scriptedClient{resp: okResp(403)} // blocked -> escalate
	tier1 := &scriptedClient{resp: okResp(200)}

	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 from escalated tier1, got %d", resp.StatusCode)
	}
	if tier0.calls != 1 || tier1.calls != 1 {
		t.Errorf("expected both tiers called once, got tier0=%d tier1=%d", tier0.calls, tier1.calls)
	}
}

func TestEscalatingClient_FirstTierTransportError_EscalatesToNext(t *testing.T) {
	t.Parallel()
	tier0 := &scriptedClient{err: errors.New("dial failed")}
	tier1 := &scriptedClient{resp: okResp(200)}

	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("expected escalation past transport error, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 from tier1, got %d", resp.StatusCode)
	}
}

func TestEscalatingClient_AllTiersEscalate_ReturnsLastResponse(t *testing.T) {
	t.Parallel()
	// Both tiers return the escalate-triggering status, so the chain is exhausted
	// and the last tier's response is surfaced rather than an error.
	tier0 := &scriptedClient{resp: okResp(403)}
	tier1 := &scriptedClient{resp: okResp(403)}

	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp == nil || resp.StatusCode != 403 {
		t.Errorf("expected last tier's 403 response when all escalate, got %+v", resp)
	}
	if tier0.calls != 1 || tier1.calls != 1 {
		t.Errorf("expected all tiers attempted, got tier0=%d tier1=%d", tier0.calls, tier1.calls)
	}
}

func TestEscalatingClient_AllTiersError_ReturnsError(t *testing.T) {
	t.Parallel()
	tier0 := &scriptedClient{err: errors.New("dial failed 0")}
	tier1 := &scriptedClient{err: errors.New("dial failed 1")}

	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})

	_, err := client.Get(context.Background(), "https://x")
	if err == nil {
		t.Fatal("expected an error when every tier fails with a transport error")
	}
}

func TestEscalatingClient_Do_NilRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{&scriptedClient{}}, escalateOn403, &noopLogger{})

	if _, err := client.Do(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestEscalatingClient_Close_ClosesAllTiers(t *testing.T) {
	t.Parallel()
	tier0 := &scriptedClient{resp: okResp(200)}
	tier1 := &scriptedClient{resp: okResp(200)}

	client, _ := webclient.NewEscalatingClient([]webclient.WebClient{tier0, tier1}, escalateOn403, &noopLogger{})

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !tier0.closed || !tier1.closed {
		t.Errorf("expected all tiers closed, got tier0=%v tier1=%v", tier0.closed, tier1.closed)
	}
}
