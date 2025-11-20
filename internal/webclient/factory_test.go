package webclient_test

import (
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// TestNewWebClient_DefaultBackend verifies that empty backend defaults to nethttp
func TestNewWebClient_DefaultBackend(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := webclient.NewWebClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create default client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	defer client.Close()
}

// TestNewWebClient_NetHTTP verifies that the factory can create a nethttp client
func TestNewWebClient_NetHTTP(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := webclient.NewWebClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create nethttp client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	defer client.Close()
}

// TestNewWebClient_ChromeDP verifies that chromedp client can be constructed
// Note: This test may be skipped in CI environments where chromedp is not fully functional
func TestNewWebClient_ChromeDP(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "chromedp",
	}
	logger := logging.NewStdoutLogger("test")

	// Chromedp may fail to initialize in headless CI environments
	client, err := webclient.NewWebClient(cfg, logger)
	if err != nil {
		t.Skipf("Skipping chromedp test: %v", err)
	}
	if client != nil {
		defer client.Close()
	}
}

// TestNewWebClient_UnknownBackend verifies that unknown backend returns error
func TestNewWebClient_UnknownBackend(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{
		WebClientBackend: "unknown",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := webclient.NewWebClient(cfg, logger)
	if err == nil {
		t.Fatal("Expected error for unknown backend, got nil")
	}
	if client != nil {
		t.Fatal("Expected nil client for unknown backend")
	}
}
