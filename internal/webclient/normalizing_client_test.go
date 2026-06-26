package webclient_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

func TestNormalizingClient_NormalizesBody(t *testing.T) {
	t.Parallel()
	inner := &scriptedClient{resp: &webclient.Response{
		StatusCode: 200,
		Body:       []byte(`<html><head></head><body><script nonce="abc">a</script><p>hi</p></body></html>`),
	}}
	client := webclient.NewNormalizingClient(inner, webclient.NewNormalizer(), &noopLogger{})

	resp, err := client.Get(context.Background(), "https://x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if bytes.Contains(resp.Body, []byte("nonce")) {
		t.Errorf("expected normalized body without nonce, got %s", resp.Body)
	}
	if !bytes.Contains(resp.Body, []byte("hi")) {
		t.Errorf("expected real content preserved, got %s", resp.Body)
	}
}

func TestNormalizingClient_PropagatesFetchError(t *testing.T) {
	t.Parallel()
	inner := &scriptedClient{err: errors.New("boom")}
	client := webclient.NewNormalizingClient(inner, webclient.NewNormalizer(), &noopLogger{})

	if _, err := client.Get(context.Background(), "https://x"); err == nil {
		t.Fatal("expected the inner fetch error to propagate")
	}
}

func TestNormalizingClient_DelegatesClose(t *testing.T) {
	t.Parallel()
	inner := &scriptedClient{resp: &webclient.Response{StatusCode: 200, Body: []byte("<p>x</p>")}}
	client := webclient.NewNormalizingClient(inner, webclient.NewNormalizer(), &noopLogger{})

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !inner.closed {
		t.Error("expected Close to delegate to the inner client")
	}
}
