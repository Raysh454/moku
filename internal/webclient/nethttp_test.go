package webclient_test

import (
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// TestNewNetHTTPClient_Construct verifies that NewNetHTTPClient returns a non-nil client
func TestNewNetHTTPClient_Construct(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := logging.NewStdoutLogger("nethttp-test")

	client, err := webclient.NewNetHTTPClient(cfg, logger)
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
	logger := logging.NewStdoutLogger("nethttp-test")

	client, err := webclient.NewNetHTTPClient(cfg, logger)
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
