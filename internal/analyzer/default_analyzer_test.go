package analyzer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
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

// TestNewDefaultAnalyzer_Construct verifies that NewDefaultAnalyzer returns a non-nil analyzer
func TestNewDefaultAnalyzer_Construct(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	analyzer, err := analyzer.NewDefaultAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewDefaultAnalyzer returned error: %v", err)
	}
	if analyzer == nil {
		t.Fatal("NewDefaultAnalyzer returned nil analyzer")
	}
	defer analyzer.Close()
}

// TestNewDefaultAnalyzer_WithCustomClient verifies that a custom *http.Client can be injected
func TestNewDefaultAnalyzer_WithCustomClient(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}
	customClient := &http.Client{}

	analyzer, err := analyzer.NewDefaultAnalyzer(cfg, logger, customClient)
	if err != nil {
		t.Fatalf("NewDefaultAnalyzer returned error: %v", err)
	}
	if analyzer == nil {
		t.Fatal("NewDefaultAnalyzer returned nil analyzer")
	}
	defer analyzer.Close()
}

// TestDefaultAnalyzer_Analyze verifies that Analyze can fetch and analyze a URL
func TestDefaultAnalyzer_Analyze(t *testing.T) {
	t.Parallel()
	
	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test Content</body></html>"))
	}))
	defer testServer.Close()

	cfg := &app.Config{}
	logger := &noopLogger{}

	analyzer, err := analyzer.NewDefaultAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewDefaultAnalyzer returned error: %v", err)
	}
	defer analyzer.Close()

	ctx := context.Background()
	resp, err := analyzer.Analyze(ctx, testServer.URL)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("Analyze returned nil response")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

// TestDefaultAnalyzer_Close verifies that Close() does not panic and returns nil
func TestDefaultAnalyzer_Close(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	analyzer, err := analyzer.NewDefaultAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewDefaultAnalyzer returned error: %v", err)
	}

	// Close should not panic and should return nil error
	err = analyzer.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
