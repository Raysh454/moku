package tracker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
)

func TestNewInMemoryTracker_Constructable(t *testing.T) {
	t.Parallel()
	cfg := &tracker.Config{}
	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := tracker.NewInMemoryTracker(cfg, logger)
	if err != nil {
		t.Fatalf("NewInMemoryTracker returned error: %v", err)
	}
	defer tr.Close()

	// Methods return ErrNotImplemented for now — assert that behavior so tests are explicit.
	_, err = tr.Commit(context.Background(), nil, "msg", "author")
	if err == nil {
		t.Fatalf("expected ErrNotImplemented from Commit, got nil")
	}
	if !errors.Is(err, tracker.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}
