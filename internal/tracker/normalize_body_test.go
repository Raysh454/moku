package tracker_test

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
)

// nonceBody returns a document whose only varying part is a script CSP nonce.
func nonceBody(nonce string) []byte {
	return []byte(`<html><head></head><body><script nonce="` + nonce +
		`">console.log(1)</script><p>stable content</p></body></html>`)
}

func diffNonceChange(t *testing.T, normalizeBody bool) int {
	t.Helper()
	dir := t.TempDir()
	logger := logging.NewStdoutLogger("normalize-body-test")
	tr, err := tracker.NewSQLiteTracker(&tracker.Config{
		StoragePath:   dir,
		ProjectID:     "normalize-body-test",
		NormalizeBody: normalizeBody,
	}, logger)
	if err != nil {
		t.Fatalf("NewSQLiteTracker: %v", err)
	}
	t.Cleanup(func() { tr.Close() })

	ctx := context.Background()
	v1, err := tr.Commit(ctx, &models.Snapshot{URL: "https://example.com", StatusCode: 200, Body: nonceBody("aaa111")}, "v1", "test")
	if err != nil {
		t.Fatalf("Commit v1: %v", err)
	}
	v2, err := tr.Commit(ctx, &models.Snapshot{URL: "https://example.com", StatusCode: 200, Body: nonceBody("bbb222")}, "v2", "test")
	if err != nil {
		t.Fatalf("Commit v2: %v", err)
	}

	diff, err := tr.DiffVersions(ctx, v1.Version.ID, v2.Version.ID)
	if err != nil {
		t.Fatalf("DiffVersions: %v", err)
	}
	chunks := 0
	for _, f := range diff.Files {
		chunks += len(f.BodyDiff.Chunks)
	}
	return chunks
}

func TestSQLiteTracker_NormalizeBody_NonceOnlyChangeProducesNoBodyDiff(t *testing.T) {
	t.Parallel()
	if chunks := diffNonceChange(t, true); chunks != 0 {
		t.Errorf("expected a nonce-only change to produce no body diff with normalization on, got %d chunks", chunks)
	}
}

func TestSQLiteTracker_NormalizeBody_Off_NonceChangeStillDiffs(t *testing.T) {
	t.Parallel()
	if chunks := diffNonceChange(t, false); chunks == 0 {
		t.Error("expected a nonce change to show a body diff when normalization is off (proves the flag gates it)")
	}
}
