package indexer_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver; replace if you prefer mattn/go-sqlite3

	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
)

// migrationSQL is the migration used by the index tests. Keep in sync with your real migration.
const migrationSQL = `
CREATE TABLE IF NOT EXISTS endpoints (
  id TEXT PRIMARY KEY,
  raw_url TEXT NOT NULL,
  canonical_url TEXT NOT NULL UNIQUE,
  host TEXT NOT NULL,
  path TEXT NOT NULL,
  first_discovered_at INTEGER NOT NULL,
  last_discovered_at INTEGER NOT NULL,
  last_fetched_version TEXT,
  last_fetched_at INTEGER,
  status TEXT,
  discovery_source TEXT,
  meta TEXT
);

CREATE INDEX IF NOT EXISTS idx_endpoints_host ON endpoints(host);
CREATE INDEX IF NOT EXISTS idx_endpoints_status ON endpoints(status);
CREATE INDEX IF NOT EXISTS idx_endpoints_last_discovered_at ON endpoints(last_discovered_at);
`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Use an in-memory DB for tests. Use shared cache if you need multiple connections.
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// serialize access to avoid SQLITE deadlocks in concurrent writers
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// increase pragmas for tests (optional)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		// Not fatal, but log for debugging
		t.Logf("pragmas: %v", err)
	}
	if _, err := db.Exec(migrationSQL); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	return db
}

func countEndpoints(t *testing.T, db *sql.DB) int {
	t.Helper()
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM endpoints`).Scan(&cnt); err != nil {
		t.Fatalf("count endpoints: %v", err)
	}
	return cnt
}

func getEndpointByCanonical(t *testing.T, db *sql.DB, canonical string) (map[string]any, error) {
	t.Helper()
	row := db.QueryRow(`SELECT id, raw_url, canonical_url, host, path, first_discovered_at, last_discovered_at, last_fetched_version, last_fetched_at, status, discovery_source, meta FROM endpoints WHERE canonical_url = ? LIMIT 1`, canonical)
	var id, raw, canon, host, pathStr, lastFetchedVersion, status, source, meta sql.NullString
	var firstDisc, lastDisc, lastFetchedAt sql.NullInt64
	if err := row.Scan(&id, &raw, &canon, &host, &pathStr, &firstDisc, &lastDisc, &lastFetchedVersion, &lastFetchedAt, &status, &source, &meta); err != nil {
		return nil, err
	}
	out := map[string]any{}
	out["id"] = id.String
	out["raw_url"] = raw.String
	out["canonical_url"] = canon.String
	out["host"] = host.String
	out["path"] = pathStr.String
	out["first_discovered_at"] = firstDisc.Int64
	out["last_discovered_at"] = lastDisc.Int64
	out["last_fetched_version"] = lastFetchedVersion.String
	out["last_fetched_at"] = lastFetchedAt.Int64
	out["status"] = status.String
	out["discovery_source"] = source.String
	out["meta"] = meta.String
	return out, nil
}

func TestAddEndpoints_Dedupe(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	logger := logging.NewStdoutLogger("index_test")
	opts := utils.CanonicalizeOptions{
		DefaultScheme:          "http",
		StripTrailingSlash:     true,
		DropTrackingParams:     true,
		TrackingParamAllowlist: nil,
	}
	ix := indexer.NewIndex(db, logger, opts)

	ctx := context.Background()

	// Two raw forms that should canonicalize to the same canonical URL
	a := "HTTP://Example.COM:80/foo/../bar/?b=2&a=1#frag"
	b := "http://example.com/bar?a=1&b=2"

	news, err := ix.AddEndpoints(ctx, []string{a, b}, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}
	if len(news) != 1 {
		t.Fatalf("expected 1 new canonical, got %d: %v", len(news), news)
	}

	// confirm DB has exactly one row
	if got := countEndpoints(t, db); got != 1 {
		t.Fatalf("expected 1 endpoint in DB, got %d", got)
	}

	// Re-run with a slightly different discovery (same canonical again)
	time.Sleep(1 * time.Second) // ensure timestamp difference
	news2, err := ix.AddEndpoints(ctx, []string{b}, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints 2 error: %v", err)
	}
	if len(news2) != 0 {
		t.Fatalf("expected 0 new canonicals on second add, got %d: %v", len(news2), news2)
	}

	// verify last_discovered_at was updated (it should be greater than or equal to first)
	canon := news[0]
	row, err := getEndpointByCanonical(t, db, canon)
	if err != nil {
		t.Fatalf("getEndpointByCanonical: %v", err)
	}
	first := row["first_discovered_at"].(int64)
	last := row["last_discovered_at"].(int64)
	if last < first {
		t.Fatalf("last_discovered_at not updated: first=%d last=%d", first, last)
	}
}

func TestMarkPending_MarkFetched_List(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	logger := logging.NewStdoutLogger("index_test")
	opts := utils.CanonicalizeOptions{
		DefaultScheme:      "https",
		StripTrailingSlash: true,
	}
	ix := indexer.NewIndex(db, logger, opts)

	ctx := context.Background()

	url := "https://example.com/some/page?utm_source=x"
	news, err := ix.AddEndpoints(ctx, []string{url}, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}
	if len(news) != 1 {
		t.Fatalf("expected 1 new canonical, got %d", len(news))
	}
	canonical := news[0]

	// Mark pending
	if err := ix.MarkPending(ctx, canonical); err != nil {
		t.Fatalf("MarkPending: %v", err)
	}

	pending, err := ix.ListEndpoints(ctx, "pending", 10)
	if err != nil {
		t.Fatalf("ListEndpoints pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending endpoint, got %d", len(pending))
	}
	if pending[0].CanonicalURL != canonical {
		t.Fatalf("pending canonical mismatch: want %s got %s", canonical, pending[0].CanonicalURL)
	}

	// Mark fetched
	versionID := "version-123"
	now := time.Now()
	if err := ix.MarkFetched(ctx, canonical, versionID, now); err != nil {
		t.Fatalf("MarkFetched: %v", err)
	}

	fetched, err := ix.ListEndpoints(ctx, "fetched", 10)
	if err != nil {
		t.Fatalf("ListEndpoints fetched: %v", err)
	}
	if len(fetched) != 1 {
		t.Fatalf("expected 1 fetched endpoint, got %d", len(fetched))
	}
	if fetched[0].LastFetchedVersion != versionID {
		t.Fatalf("LastFetchedVersion mismatch: want %s got %s", versionID, fetched[0].LastFetchedVersion)
	}
	if fetched[0].Status != "fetched" {
		t.Fatalf("expected status fetched, got %s", fetched[0].Status)
	}
}

func TestConcurrentAddEndpoints(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	logger := logging.NewStdoutLogger("index_test")
	opts := utils.CanonicalizeOptions{
		DefaultScheme:      "http",
		StripTrailingSlash: true,
		DropTrackingParams: true,
	}
	ix := indexer.NewIndex(db, logger, opts)

	ctx := context.Background()

	// Create many URLs with intentional duplicates
	base := "http://example.com"
	urls := []string{
		base + "/a",
		base + "/b",
		base + "/a/",             // duplicate of /a with trailing slash
		base + "/a?utm_source=1", // duplicate if tracking params are dropped
		base + "/c",
		base + "/b?b=2&a=1", // different query -> distinct (unless order-normalized)
		base + "/b?a=1&b=2", // same as above but diff order -> canonicalize equal
	}
	// Repeat set to cause concurrent inserts
	iterations := 10

	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := ix.AddEndpoints(ctx, urls, fmt.Sprintf("spider-%d", i))
			if err != nil {
				t.Errorf("goroutine %d: AddEndpoints error: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Count unique canonical endpoints
	cnt := countEndpoints(t, db)
	// Determine expected unique count:
	// /a, /b, /c, /b?a=1&b=2  => 4
	if cnt != 4 {
		t.Fatalf("expected 4 unique endpoints after concurrent adds, got %d", cnt)
	}
}
