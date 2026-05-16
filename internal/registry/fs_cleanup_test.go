package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/logging"
)

func TestRemoveDirBestEffort_NoOp_WhenPathEmpty(t *testing.T) {
	// Should simply return without touching anything.
	removeDirBestEffort("", logging.NewStdoutLogger("test"))
}

func TestRemoveDirBestEffort_DeletesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "to-delete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	removeDirBestEffort(dir, logging.NewStdoutLogger("test"))

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir still present after removeDirBestEffort (stat err = %v)", err)
	}
}

func TestRemoveDirBestEffort_RenamesAside_WhenRemoveAllAlwaysFails(t *testing.T) {
	// Simulate an OS lock by stubbing RemoveAll to always fail.
	origRemove := fsRemoveAll
	origRename := fsRename
	origBackoffs := removeDirRetryBackoffs
	t.Cleanup(func() {
		fsRemoveAll = origRemove
		fsRename = origRename
		removeDirRetryBackoffs = origBackoffs
	})
	// Speed the test up by collapsing the backoffs.
	removeDirRetryBackoffs = []time.Duration{0, 0, 0}

	fsRemoveAll = func(_ string) error { return errors.New("simulated lock") }

	var renamedFrom, renamedTo string
	fsRename = func(from, to string) error {
		renamedFrom, renamedTo = from, to
		return nil
	}

	const path = "C:\\fake\\website"
	removeDirBestEffort(path, logging.NewStdoutLogger("test"))

	if renamedFrom != path {
		t.Errorf("rename source = %q, want %q", renamedFrom, path)
	}
	if !strings.HasPrefix(renamedTo, path+".deleted-") {
		t.Errorf("rename target = %q, want prefix %q", renamedTo, path+".deleted-")
	}
}

func TestRemoveDirBestEffort_SwallowsRenameError(t *testing.T) {
	origRemove := fsRemoveAll
	origRename := fsRename
	origBackoffs := removeDirRetryBackoffs
	t.Cleanup(func() {
		fsRemoveAll = origRemove
		fsRename = origRename
		removeDirRetryBackoffs = origBackoffs
	})
	removeDirRetryBackoffs = []time.Duration{0, 0, 0}

	fsRemoveAll = func(_ string) error { return errors.New("locked") }
	fsRename = func(_, _ string) error { return errors.New("rename also failed") }

	// Must not panic and must not propagate an error (helper signature is void).
	removeDirBestEffort("C:\\fake\\website", logging.NewStdoutLogger("test"))
}
