package tracker

import (
	"context"
	"errors"

	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
)

// ErrNotImplemented is returned by scaffold methods that are yet to be implemented.
var ErrNotImplemented = errors.New("tracker: not implemented (scaffold)")

// NewInMemoryTracker constructs a simple in-memory tracker scaffold.
// The returned instance implements interfaces.Tracker but methods return ErrNotImplemented
// until you add concrete behavior (persisting snapshots, diffs, etc).
func NewInMemoryTracker(cfg *Config, logger interfaces.Logger) (interfaces.Tracker, error) {
	// Keep the constructor minimal; callers must pass a non-nil logger.
	if cfg == nil {
		cfg = &Config{}
	}
	if logger == nil {
		return nil, errors.New("tracker: nil logger provided")
	}

	// TODO: initialize any internal in-memory structures here when implementing.
	return &inMemoryTracker{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// inMemoryTracker is a minimal scaffold implementation.
type inMemoryTracker struct {
	cfg    *Config
	logger interfaces.Logger
}

// Ensure inMemoryTracker implements interfaces.Tracker at compile-time.
var _ interfaces.Tracker = (*inMemoryTracker)(nil)

func (t *inMemoryTracker) Commit(ctx context.Context, snapshot *model.Snapshot, message string, author string) (*model.Version, error) {
	// TODO: record snapshot and create a Version record.
	return nil, ErrNotImplemented
}

func (t *inMemoryTracker) Diff(ctx context.Context, baseID, headID string) (*model.DiffResult, error) {
	// TODO: compute textual/DOM diffs between snapshots.
	return nil, ErrNotImplemented
}

func (t *inMemoryTracker) Get(ctx context.Context, versionID string) (*model.Snapshot, error) {
	// TODO: return the snapshot for versionID.
	return nil, ErrNotImplemented
}

func (t *inMemoryTracker) List(ctx context.Context, limit int) ([]*model.Version, error) {
	// TODO: return recent versions up to limit.
	return nil, ErrNotImplemented
}

func (t *inMemoryTracker) Checkout(ctx context.Context, versionID string) error {
	// TODO: restore working tree from versionID.
	return ErrNotImplemented
}

func (t *inMemoryTracker) Close() error {
	// No resources in scaffold; return nil.
	return nil
}
