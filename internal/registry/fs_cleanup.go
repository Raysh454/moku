package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/raysh454/moku/internal/logging"
)

// Filesystem operations are taken via package-level vars so tests can inject
// failures (e.g. a permanently-locked file) without spawning a real Windows
// file lock. Production code uses the os.* implementations.
var (
	fsRemoveAll = os.RemoveAll
	fsRename    = os.Rename
)

// removeDirRetryBackoffs lists the delays between failed RemoveAll attempts.
// Three retries absorb the brief Windows handle-release lag we observed when
// reproducing issue #17 (SQLite `moku.db` still locked moments after Close).
var removeDirRetryBackoffs = []time.Duration{
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
}

// removeDirBestEffort tries hard to remove path. It first calls RemoveAll;
// on transient failures (typical on Windows when a handle is still being
// released) it retries with backoff. If removal still fails it renames the
// directory aside so a recreate at the same logical name never silently
// picks up stale state. Errors are logged but never propagated — the caller
// has already removed the corresponding DB row, so the user-visible operation
// must succeed.
func removeDirBestEffort(path string, logger logging.Logger) {
	if path == "" {
		return
	}

	if err := tryRemoveAll(path); err == nil {
		return
	}

	for _, backoff := range removeDirRetryBackoffs {
		time.Sleep(backoff)
		if err := tryRemoveAll(path); err == nil {
			return
		}
	}

	quarantined := path + ".deleted-" + time.Now().UTC().Format("20060102T150405.000Z")
	if err := fsRename(path, quarantined); err != nil {
		if logger != nil {
			logger.Warn("failed to remove or quarantine directory",
				logging.Field{Key: "path", Value: path},
				logging.Field{Key: "rename_error", Value: err.Error()})
		}
		return
	}
	if logger != nil {
		logger.Warn("directory could not be removed; renamed aside",
			logging.Field{Key: "original", Value: path},
			logging.Field{Key: "quarantined", Value: quarantined})
	}
}

// tryRemoveAll wraps fsRemoveAll and treats "already gone" as success.
func tryRemoveAll(path string) error {
	err := fsRemoveAll(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove %s: %w", path, err)
}
