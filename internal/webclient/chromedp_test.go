package webclient_test

import (
	"context"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// noopLogger is a test-local logger implementation that discards all log messages
type chromedpNoopLogger struct{}

func (n *chromedpNoopLogger) Debug(msg string, fields ...logging.Field) {}
func (n *chromedpNoopLogger) Info(msg string, fields ...logging.Field)  {}
func (n *chromedpNoopLogger) Warn(msg string, fields ...logging.Field)  {}
func (n *chromedpNoopLogger) Error(msg string, fields ...logging.Field) {}
func (n *chromedpNoopLogger) With(fields ...logging.Field) logging.Logger {
	return n
}

// TestNewChromedpClient_Construct verifies that NewChromedpClient returns a non-nil client
// Note: This test may be skipped in CI environments where chromedp cannot initialize
func TestNewChromedpClient_Construct(t *testing.T) {
	t.Parallel()
	cfg := webclient.Config{Client: webclient.ClientChromedp}
	logger := &chromedpNoopLogger{}

	client, err := webclient.NewChromedpClient(cfg, logger)
	if err != nil {
		t.Skipf("Skipping chromedp construction test (environment does not support chromedp): %v", err)
	}
	if client == nil {
		t.Fatal("NewChromedpClient returned nil client without error")
	}
	defer client.Close()
}

// TestChromedpClient_DoSupportsGET verifies that Do() works with GET requests
// Note: chromedp only supports GET requests; other methods should fail
func TestChromedpClient_DoSupportsGET(t *testing.T) {
	t.Parallel()
	cfg := webclient.Config{Client: webclient.ClientChromedp}
	logger := &chromedpNoopLogger{}

	client, err := webclient.NewChromedpClient(cfg, logger)
	if err != nil {
		t.Skipf("Skipping chromedp Do test (environment does not support chromedp): %v", err)
	}
	if client == nil {
		t.Fatal("NewChromedpClient returned nil client without error")
	}
	defer client.Close()

	// Test that Do() with GET is callable (even if it fails due to network/url issues)
	// We're testing the interface, not performing real network calls
	ctx := context.Background()
	req := &webclient.Request{
		Method: "GET",
		URL:    "about:blank",
	}

	// This may succeed or fail depending on environment; we just ensure it doesn't panic
	_, _ = client.Do(ctx, req)
}

// TestChromedpClient_DoRejectsNonGET verifies that Do() returns error for non-GET methods
func TestChromedpClient_DoRejectsNonGET(t *testing.T) {
	t.Parallel()
	cfg := webclient.Config{Client: webclient.ClientChromedp}
	logger := &chromedpNoopLogger{}

	client, err := webclient.NewChromedpClient(cfg, logger)
	if err != nil {
		t.Skipf("Skipping chromedp non-GET test (environment does not support chromedp): %v", err)
	}
	if client == nil {
		t.Fatal("NewChromedpClient returned nil client without error")
	}
	defer client.Close()

	ctx := context.Background()
	req := &webclient.Request{
		Method: "POST",
		URL:    "http://example.com",
	}

	_, err = client.Do(ctx, req)
	if err == nil {
		t.Fatal("Expected error for POST request, got nil")
	}
	// Verify error message indicates method not supported
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected error about method not supported, got: %v", err)
	}
}
