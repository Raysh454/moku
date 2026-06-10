package server

import (
	"context"
	"database/sql"
	"testing"
)

// The shared registry database serves every concurrent HTTP request, so its
// busy_timeout and WAL pragmas must hold on every connection the pool opens.
// Two connections are held simultaneously to force two distinct connections.
func TestOpenRegistryDB_AppliesPragmasToEveryPoolConnection(t *testing.T) {
	db, err := openRegistryDB(t.TempDir())
	if err != nil {
		t.Fatalf("openRegistryDB returned error: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
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

		var journalMode string
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("connection %d: query journal_mode: %v", i+1, err)
		}
		if journalMode != "wal" {
			t.Errorf("connection %d: journal_mode = %q, want \"wal\"", i+1, journalMode)
		}
	}
}
