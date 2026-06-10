package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/server"
)

// recordingLogger captures every logged field so tests can assert on what the
// request logger does and does not emit. Safe for concurrent use.
type recordingLogger struct {
	mu      sync.Mutex
	entries []recordedEntry
}

type recordedEntry struct {
	level  string
	msg    string
	fields []logging.Field
}

func (l *recordingLogger) record(level, msg string, fields []logging.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := append([]logging.Field(nil), fields...)
	l.entries = append(l.entries, recordedEntry{level: level, msg: msg, fields: cp})
}

func (l *recordingLogger) Debug(msg string, fields ...logging.Field) { l.record("debug", msg, fields) }
func (l *recordingLogger) Info(msg string, fields ...logging.Field)  { l.record("info", msg, fields) }
func (l *recordingLogger) Warn(msg string, fields ...logging.Field)  { l.record("warn", msg, fields) }
func (l *recordingLogger) Error(msg string, fields ...logging.Field) { l.record("error", msg, fields) }
func (l *recordingLogger) With(_ ...logging.Field) logging.Logger    { return l }

// allFieldValues returns the stringified value of every captured field across
// every logged entry.
func (l *recordingLogger) allFieldValues() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.entries {
		for _, f := range e.fields {
			out = append(out, fmt.Sprintf("%v", f.Value))
		}
	}
	return out
}

// hasFieldKey reports whether any captured entry logged a field with the key.
func (l *recordingLogger) hasFieldKey(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		for _, f := range e.fields {
			if f.Key == key {
				return true
			}
		}
	}
	return false
}

func newServerWithLogger(t *testing.T, logger logging.Logger) *server.Server {
	t.Helper()
	cfg := server.Config{
		AppConfig: &app.Config{StorageRoot: t.TempDir()},
		Logger:    logger,
	}
	s, err := server.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestServeHTTP_DoesNotLogRequestBody(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	logger := &recordingLogger{}
	s := newServerWithLogger(t, logger)

	const marker = "UNIQUE-BODY-MARKER-9f3a"
	body := strings.NewReader(`{"slug":"p","name":"` + marker + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	for _, v := range logger.allFieldValues() {
		if strings.Contains(v, marker) {
			t.Fatalf("request body marker %q leaked into a logged field: %q", marker, v)
		}
	}
	if !logger.hasFieldKey("content_length") {
		t.Fatal("expected a content_length field to be logged for POST")
	}
}

func TestServeHTTP_RedactsTokenQueryParam(t *testing.T) {
	t.Setenv("MOKU_API_TOKEN", "")
	logger := &recordingLogger{}
	s := newServerWithLogger(t, logger)

	const secret = "supersecret-marker"
	// The SSE handler streams until its request context is canceled. Pre-cancel
	// the context so the handler returns immediately; the request is logged
	// before any streaming begins, which is what we are asserting on.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/jobs/events?token="+secret, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	for _, v := range logger.allFieldValues() {
		if strings.Contains(v, secret) {
			t.Fatalf("token query value %q leaked into a logged field: %q", secret, v)
		}
	}
}
