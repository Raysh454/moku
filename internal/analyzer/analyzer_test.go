package analyzer_test

import (
	"context"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/logging"
)

// noopLogger discards all log messages. Shared by every analyzer test.
type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...logging.Field) {}
func (noopLogger) Info(msg string, fields ...logging.Field)  {}
func (noopLogger) Warn(msg string, fields ...logging.Field)  {}
func (noopLogger) Error(msg string, fields ...logging.Field) {}
func (noopLogger) With(fields ...logging.Field) logging.Logger {
	return noopLogger{}
}

// analyzerFactory constructs a fresh analyzer for a single contract sub-test.
// The factory is responsible for any fakes it needs and for failing the test
// fast if construction cannot complete.
type analyzerFactory func(t *testing.T) analyzer.Analyzer

// runAnalyzerContract exercises the black-box invariants every analyzer.Analyzer
// implementation must satisfy regardless of backend. Burp and ZAP adapters added
// in future plans re-use this suite by passing their own factory.
func runAnalyzerContract(t *testing.T, expectedBackend analyzer.Backend, newA analyzerFactory) {
	t.Helper()

	t.Run("Name returns the expected backend", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		if got := a.Name(); got != expectedBackend {
			t.Errorf("Name() = %q, want %q", got, expectedBackend)
		}
	})

	t.Run("Capabilities does not panic", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		_ = a.Capabilities()
	})

	t.Run("SubmitScan with nil request returns error", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		if _, err := a.SubmitScan(context.Background(), nil); err == nil {
			t.Error("SubmitScan(nil) returned no error; want error")
		}
	})

	t.Run("SubmitScan with empty URL returns error", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		if _, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: ""}); err == nil {
			t.Error("SubmitScan(empty URL) returned no error; want error")
		}
	})

	t.Run("SubmitScan with valid request returns non-empty job ID quickly", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		start := time.Now()
		jobID, err := a.SubmitScan(context.Background(), &analyzer.ScanRequest{URL: "https://example.com/"})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("SubmitScan: %v", err)
		}
		if jobID == "" {
			t.Fatal("SubmitScan returned empty job ID")
		}
		// The submit call itself must not block on the scan — industry scanners
		// all return a task ID immediately even when the scan takes minutes.
		if elapsed > 500*time.Millisecond {
			t.Errorf("SubmitScan took %s; expected to return quickly (<500ms)", elapsed)
		}
	})

	t.Run("GetScan with empty job ID returns error", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		if _, err := a.GetScan(context.Background(), ""); err == nil {
			t.Error("GetScan(empty) returned no error; want error")
		}
	})

	t.Run("GetScan with unknown job ID returns error", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		if _, err := a.GetScan(context.Background(), "unknown-job-id-that-cannot-exist"); err == nil {
			t.Error("GetScan(unknown) returned no error; want error")
		}
	})

	t.Run("ScanAndWait completes and populates Findings and Summary", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{
			Timeout:  5 * time.Second,
			Interval: 50 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("ScanAndWait: %v", err)
		}
		if result == nil {
			t.Fatal("ScanAndWait returned nil result with nil error")
		} else {
			if result.Backend != expectedBackend {
				t.Errorf("result.Backend = %q, want %q", result.Backend, expectedBackend)
			}
			if result.Status != analyzer.StatusCompleted {
				t.Errorf("result.Status = %q, want %q", result.Status, analyzer.StatusCompleted)
			}
			if result.Findings == nil {
				t.Error("result.Findings is nil; contract requires non-nil once Completed")
			}
			if result.Summary == nil {
				t.Fatal("result.Summary is nil; contract requires non-nil once Completed")
			} else {
				if result.Summary.Total != len(result.Findings) {
					t.Errorf("Summary.Total = %d, want %d (= len(Findings))", result.Summary.Total, len(result.Findings))
				}
				severitySum := result.Summary.Info + result.Summary.Low + result.Summary.Medium + result.Summary.High + result.Summary.Critical
				if severitySum != result.Summary.Total {
					t.Errorf("per-severity counts sum to %d, want %d (= Summary.Total)", severitySum, result.Summary.Total)
				}
			}
		}
	})

	t.Run("ScanAndWait honors context cancellation", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // pre-cancel before calling

		_, err := a.ScanAndWait(ctx, &analyzer.ScanRequest{URL: "https://example.com/"}, analyzer.PollOptions{
			Timeout:  5 * time.Second,
			Interval: 50 * time.Millisecond,
		})
		if err == nil {
			t.Error("ScanAndWait with pre-canceled context returned no error; want ctx.Err()")
		}
	})

	t.Run("Health returns a status", func(t *testing.T) {
		a := newA(t)
		defer func() { _ = a.Close() }()
		status, err := a.Health(context.Background())
		if err != nil {
			t.Fatalf("Health: %v", err)
		}
		if status == "" {
			t.Error("Health returned empty status")
		}
	})

	t.Run("Close is idempotent", func(t *testing.T) {
		a := newA(t)
		if err := a.Close(); err != nil {
			t.Fatalf("first Close: %v", err)
		}
		if err := a.Close(); err != nil {
			t.Errorf("second Close: %v (Close must be idempotent)", err)
		}
	})
}
