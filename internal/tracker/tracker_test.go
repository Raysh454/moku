package tracker

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/logging"
)

func TestNewInMemoryTracker_Constructable(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	logger := logging.NewStdoutLogger("tracker-test")
	tr, err := NewInMemoryTracker(cfg, logger)
	if err != nil {
		t.Fatalf("NewInMemoryTracker returned error: %v", err)
	}
	defer tr.Close()

	// Methods return ErrNotImplemented for now — assert that behavior so tests are explicit.
	_, err = tr.Commit(context.Background(), nil, "msg", "author")
	if err == nil {
		t.Fatalf("expected ErrNotImplemented from Commit, got nil")
	}
}
