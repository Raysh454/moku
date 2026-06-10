package harness

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// Process is a spawned binary under test. It captures combined output so a
// failing scenario can show what the subprocess said, and it is always killed
// on test cleanup.
type Process struct {
	cmd    *exec.Cmd
	output *lockedBuffer
}

// ProcessSpec describes how to launch a binary: the executable path, its
// arguments, and extra environment entries appended after the inherited
// environment (so they win on duplicates).
type ProcessSpec struct {
	Path string
	Args []string
	Env  []string
	Dir  string
}

// StartProcess launches the binary described by spec and registers cleanup
// that kills it when the test finishes.
func StartProcess(t *testing.T, spec ProcessSpec) *Process {
	t.Helper()

	output := &lockedBuffer{}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("harness: start %s: %v", spec.Path, err)
	}

	t.Cleanup(func() {
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	return &Process{cmd: cmd, output: output}
}

// Exited reports whether the process has already terminated.
func (p *Process) Exited() bool {
	return p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited()
}

// Logs returns everything the process has written to stdout/stderr so far.
func (p *Process) Logs() string {
	return p.output.String()
}

// lockedBuffer is a goroutine-safe bytes.Buffer; exec.Cmd writes from a
// different goroutine than the test reads from.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
