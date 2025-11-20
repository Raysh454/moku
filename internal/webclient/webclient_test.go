package webclient

import (
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
)

// TestFactoryNetHTTP verifies that the factory can create a nethttp client
func TestFactoryNetHTTP(t *testing.T) {
	cfg := &app.Config{
		WebClientBackend: "nethttp",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := NewWebClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create nethttp client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	defer client.Close()
}

// TestFactoryDefaultBackend verifies that empty backend defaults to nethttp
func TestFactoryDefaultBackend(t *testing.T) {
	cfg := &app.Config{
		WebClientBackend: "",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := NewWebClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create default client: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	defer client.Close()
}

// TestFactoryUnknownBackend verifies that unknown backend returns error
func TestFactoryUnknownBackend(t *testing.T) {
	cfg := &app.Config{
		WebClientBackend: "unknown",
	}
	logger := logging.NewStdoutLogger("test")

	client, err := NewWebClient(cfg, logger)
	if err == nil {
		t.Fatal("Expected error for unknown backend, got nil")
	}
	if client != nil {
		t.Fatal("Expected nil client for unknown backend")
	}
}

// TestChromeDPClientConstruction verifies that chromedp client can be constructed
// Note: This test may be skipped in CI environments where chromedp is not fully functional
func TestChromeDPClientConstruction(t *testing.T) {
	cfg := &app.Config{
		WebClientBackend: "chromedp",
	}
	logger := logging.NewStdoutLogger("test")

	// Chromedp may fail to initialize in headless CI environments
	client, err := NewWebClient(cfg, logger)
	if err != nil {
		t.Skipf("Skipping chromedp test: %v", err)
	}
	if client != nil {
		defer client.Close()
	}
}

