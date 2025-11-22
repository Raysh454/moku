package fetcher_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/model"
)

// mockTracker implements TrackerCommitter for testing
type mockTracker struct {
	commits []*model.Snapshot
}

func (m *mockTracker) Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.Version, error) {
	m.commits = append(m.commits, snapshot)
	return &model.Version{
		ID:      "test-version-id",
		Message: message,
		Author:  author,
	}, nil
}

func TestCommitResponseToTracker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupResponse func() *http.Response
		suggestedPath string
		wantErr       bool
		checkSnapshot func(t *testing.T, snap *model.Snapshot)
	}{
		{
			name: "successful commit with headers",
			setupResponse: func() *http.Response {
				body := []byte("<html><body>Test</body></html>")
				resp := &http.Response{
					StatusCode: 200,
					Header: http.Header{
						"Content-Type":  []string{"text/html"},
						"Cache-Control": []string{"no-cache"},
					},
					Body: io.NopCloser(bytes.NewReader(body)),
					Request: &http.Request{
						URL: &url.URL{
							Scheme: "https",
							Host:   "example.com",
							Path:   "/test",
						},
					},
				}
				return resp
			},
			suggestedPath: "test.html",
			wantErr:       false,
			checkSnapshot: func(t *testing.T, snap *model.Snapshot) {
				if snap.URL != "https://example.com/test" {
					t.Errorf("expected URL https://example.com/test, got %s", snap.URL)
				}
				if string(snap.Body) != "<html><body>Test</body></html>" {
					t.Errorf("unexpected body: %s", string(snap.Body))
				}
				if snap.Meta == nil {
					t.Fatal("meta should not be nil")
				}
				if snap.Meta["_status_code"] != "200" {
					t.Errorf("expected status code 200, got %s", snap.Meta["_status_code"])
				}
				if snap.Meta["_file_path"] != "test.html" {
					t.Errorf("expected file path test.html, got %s", snap.Meta["_file_path"])
				}
				if snap.Meta["_headers"] == "" {
					t.Error("headers should not be empty")
				}
			},
		},
		{
			name: "empty body",
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: 204,
					Header:     http.Header{},
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
					Request: &http.Request{
						URL: &url.URL{
							Scheme: "https",
							Host:   "example.com",
						},
					},
				}
			},
			suggestedPath: "empty.html",
			wantErr:       false,
			checkSnapshot: func(t *testing.T, snap *model.Snapshot) {
				if len(snap.Body) != 0 {
					t.Errorf("expected empty body, got %d bytes", len(snap.Body))
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			tracker := &mockTracker{}
			resp := tt.setupResponse()

			version, err := fetcher.CommitResponseToTracker(ctx, tracker, resp, tt.suggestedPath)

			if (err != nil) != tt.wantErr {
				t.Fatalf("CommitResponseToTracker() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if version == nil {
				t.Fatal("expected version, got nil")
			}

			if len(tracker.commits) != 1 {
				t.Fatalf("expected 1 commit, got %d", len(tracker.commits))
			}

			if tt.checkSnapshot != nil {
				tt.checkSnapshot(t, tracker.commits[0])
			}
		})
	}
}

func TestCommitResponseToTracker_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil tracker", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("test"))),
		}

		_, err := fetcher.CommitResponseToTracker(context.Background(), nil, resp, "test.html")
		if err == nil {
			t.Error("expected error with nil tracker")
		}
	})

	t.Run("nil response", func(t *testing.T) {
		t.Parallel()

		tracker := &mockTracker{}
		_, err := fetcher.CommitResponseToTracker(context.Background(), tracker, nil, "test.html")
		if err == nil {
			t.Error("expected error with nil response")
		}
	})
}
