package analyzer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
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
		if _, err := w.Write([]byte("<html><body>Test Content</body></html>")); err != nil {
			t.Fatalf("write: %v", err)
		}
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

// TestNewAnalyzer_Construct verifies that NewAnalyzer returns a non-nil interfaces.Analyzer
func TestNewAnalyzer_Construct(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	if a == nil {
		t.Fatal("NewAnalyzer returned nil analyzer")
	}
	defer a.Close()
}

// TestNewAnalyzer_Health verifies that Health returns "ok"
func TestNewAnalyzer_Health(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	status, err := a.Health(ctx)
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if status != "ok" {
		t.Errorf("Expected health status 'ok', got '%s'", status)
	}
}

// TestNewAnalyzer_SubmitScan verifies that SubmitScan returns a job ID
func TestNewAnalyzer_SubmitScan(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	req := &model.ScanRequest{
		URL: "https://example.com",
	}

	jobID, err := a.SubmitScan(ctx, req)
	if err != nil {
		t.Fatalf("SubmitScan returned error: %v", err)
	}
	if jobID == "" {
		t.Error("SubmitScan returned empty job ID")
	}
}

// TestNewAnalyzer_SubmitScan_NilRequest verifies that SubmitScan returns error for nil request
func TestNewAnalyzer_SubmitScan_NilRequest(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	_, err = a.SubmitScan(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil request, got nil")
	}
}

// TestNewAnalyzer_ScanAndWait verifies that ScanAndWait performs a scan
func TestNewAnalyzer_ScanAndWait(t *testing.T) {
	t.Parallel()

	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("<html><body>Test Content</body></html>")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}))
	defer testServer.Close()

	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	req := &model.ScanRequest{
		URL: testServer.URL,
	}

	result, err := a.ScanAndWait(ctx, req, 10, 1)
	if err != nil {
		t.Fatalf("ScanAndWait returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ScanAndWait returned nil result")
	}
	if result.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", result.Status)
	}
	if result.Response == nil {
		t.Error("Expected non-nil response in result")
	}
	if result.Response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.Response.StatusCode)
	}
}

// TestNewAnalyzer_GetScan verifies that GetScan returns a result
func TestNewAnalyzer_GetScan(t *testing.T) {
	t.Parallel()
	cfg := &app.Config{}
	logger := &noopLogger{}

	a, err := analyzer.NewAnalyzer(cfg, logger, nil)
	if err != nil {
		t.Fatalf("NewAnalyzer returned error: %v", err)
	}
	defer a.Close()

	ctx := context.Background()
	result, err := a.GetScan(ctx, "test-job-id")
	if err != nil {
		t.Fatalf("GetScan returned error: %v", err)
	}
	if result == nil {
		t.Fatal("GetScan returned nil result")
	}
}
