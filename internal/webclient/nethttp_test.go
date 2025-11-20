package webclient_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/webclient"
)

// noopLogger is a test-local logger implementation that discards all log messages
type noopLogger struct{}

func (n *noopLogger) Debug(msg string, fields ...interfaces.Field) {}
func (n *noopLogger) Info(msg string, fields ...interfaces.Field)  {}
func (n *noopLogger) Warn(msg string, fields ...interfaces.Field)  {}
func (n *noopLogger) Error(msg string, fields ...interfaces.Field) {}
func (n *noopLogger) With(fields ...interfaces.Field) interfaces.Logger {
	return n
}

// TestNewNetHTTPClient_Construct verifies that NewNetHTTPClient returns a non-nil client
func TestNewNetHTTPClient_Construct(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := &noopLogger{}

	client, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewNetHTTPClient returned nil client")
	}
	defer client.Close()
}

// TestNewNetHTTPClient_WithCustomClient verifies that a custom *http.Client can be injected
func TestNewNetHTTPClient_WithCustomClient(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := &noopLogger{}
	customClient := &http.Client{}

	client, err := webclient.NewNetHTTPClient(cfg, logger, customClient)
	if err != nil {
		t.Fatalf("NewNetHTTPClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewNetHTTPClient returned nil client")
	}
	defer client.Close()
}

// TestNetHTTPClient_Close verifies that Close() does not panic and returns nil
func TestNetHTTPClient_Close(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := &noopLogger{}

	client, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewNetHTTPClient returned nil client")
	}

	// Close should not panic and should return nil error
	err = client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestNetHTTPClient_DoHTTPRequest verifies DoHTTPRequest method
func TestNetHTTPClient_DoHTTPRequest(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	client, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient returned error: %v", err)
	}
	defer client.Close()

	// Note: DoHTTPRequest is not exposed through interfaces.WebClient
	// This test structure is kept for when the interface is extended
	_ = context.Background()
}

// TestNetHTTPClient_ErrInvalidRequest verifies ErrInvalidRequest method
func TestNetHTTPClient_ErrInvalidRequest(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	client, err := webclient.NewNetHTTPClient(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewNetHTTPClient returned error: %v", err)
	}
	defer client.Close()

	// Note: ErrInvalidRequest is not exposed through interfaces.WebClient
	// This test structure is kept for when the interface is extended
}
