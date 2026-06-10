package utils_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/utils"
	_ "modernc.org/sqlite"
)

// Opening a database through SQLiteDSN must apply each pragma on every
// connection the pool creates, not just the first one. Two connections are
// held simultaneously to force the pool to open two distinct connections.
func TestSQLiteDSN_AppliesPragmasToEveryPoolConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dsn_test.db")
	dsn := utils.SQLiteDSN(dbPath, "busy_timeout(5000)", "journal_mode(WAL)")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db with DSN %q: %v", dsn, err)
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

func TestSQLiteDSN_WithoutPragmasReturnsBarePath(t *testing.T) {
	if got := utils.SQLiteDSN("some.db"); got != "some.db" {
		t.Errorf("SQLiteDSN(\"some.db\") = %q, want \"some.db\"", got)
	}
}
