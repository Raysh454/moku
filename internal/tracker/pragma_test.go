package tracker_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
)

// The tracker database must enforce its correctness pragmas (busy_timeout,
// foreign_keys, WAL) on every connection the pool opens — not only on the
// connection that ran the schema setup. Two connections are held
// simultaneously to force the pool to open two distinct connections.
func TestNewSQLiteTracker_AppliesPragmasToEveryPoolConnection(t *testing.T) {
	tr, err := tracker.NewSQLiteTracker(
		&tracker.Config{StoragePath: t.TempDir(), ProjectID: "pragma-test"},
		logging.NewStdoutLogger("tracker-test"),
		nil,
	)
	if err != nil {
		t.Fatalf("NewSQLiteTracker returned error: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()
	db := tr.DB()

	conn1, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	defer conn1.Close()
	conn2, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	defer conn2.Close()

	for i, conn := range []*sql.Conn{conn1, conn2} {
		var busyTimeout int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("connection %d: query busy_timeout: %v", i+1, err)
		}
		if busyTimeout != 5000 {
			t.Errorf("connection %d: busy_timeout = %d, want 5000", i+1, busyTimeout)
		}

		var foreignKeys int
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("connection %d: query foreign_keys: %v", i+1, err)
		}
		if foreignKeys != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1 (enabled)", i+1, foreignKeys)
		}

		var journalMode string
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("connection %d: query journal_mode: %v", i+1, err)
		}
		if journalMode != "wal" {
			t.Errorf("connection %d: journal_mode = %q, want \"wal\"", i+1, journalMode)
		}
	}
}
