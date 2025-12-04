package app

import (
	"context"
	"errors"
	"time"

	"github.com/raysh454/moku/internal/cli"
	"github.com/raysh454/moku/internal/logging"
)

// Minimal orchestrator contract used for wiring in the dev branch.
// Keep very small here so tests can provide a stub; real orchestrator will
// implement a richer interface in its own package.
type Orchestrator interface {
	// SubmitJob accepts a generic job request (use concrete types later).
	SubmitJob(ctx context.Context, req map[string]any) (jobID string, err error)

	// Shutdown attempts graceful shutdown of the orchestrator.
	Shutdown(ctx context.Context) error
}

// Application is the global runtime state container.
// It holds config, parsed CLI args and the core services that are shared
// across modules (orchestrator, logger). Pass Application into modules that
// need access to the global state rather than using package-level variables.
type Application struct {
	Config *Config
	Args   *cli.CLIArgs

	// Use the shared logger interface from internal/interfaces
	Logger logging.Logger
	Orch   Orchestrator

	// internal context for cancellation / lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewApplication constructs an Application from the provided parts.
// Keep the constructor simple: pass already-constructed parts so this function
// is easy to test and does not import heavy dependencies.
func NewApplication(cfg *Config, args *cli.CLIArgs, logger logging.Logger, orch Orchestrator) *Application {
	ctx, cancel := context.WithCancel(context.Background())

	return &Application{
		Config: cfg,
		Args:   args,
		Logger: logger,
		Orch:   orch,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins any background work the Application needs. For now this is
// intentionally minimal ("keep it simple stupid"): we do not start an HTTP
// server or spawn long-lived goroutines here. Start is a no-op aside from
// optional logging so we can wire components without side-effects.
func (a *Application) Start() error {
	if a == nil {
		return errors.New("application is nil")
	}
	if a.Logger != nil {
		a.Logger.Info("application starting", logging.Field{Key: "target", Value: a.Args.Target})
	}
	return nil
}

// Shutdown attempts a graceful shutdown, delegating to the orchestrator first.
func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return errors.New("application is nil")
	}
	if a.Logger != nil {
		a.Logger.Info("application shutdown initiated")
	}

	// Ask orchestrator to shut down first with a bounded timeout.
	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if a.Orch != nil {
		if err := a.Orch.Shutdown(shutdownCtx); err != nil {
			if a.Logger != nil {
				a.Logger.Info("orchestrator shutdown returned error", logging.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// cancel internal ctx to signal local components/tests
	a.cancel()

	return nil
}
