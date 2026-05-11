package indexer_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver; replace if you prefer mattn/go-sqlite3

	"github.com/raysh454/moku/internal/filter"
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

func TestListEndpointsFiltered(t *testing.T) {
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

	// Add various endpoints including some that should be filtered
	urls := []string{
		"http://example.com/page.html",
		"http://example.com/image.jpg",
		"http://example.com/logo.png",
		"http://example.com/script.js",
		"http://example.com/assets/style.css",
		"http://example.com/api/data",
	}

	_, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	// Create filter config that skips .jpg and .png
	filterConfig := &filter.Config{
		SkipExtensions: []string{".jpg", ".png"},
	}

	// List endpoints with filter
	filtered, err := ix.ListEndpointsFiltered(ctx, "*", 100, filterConfig)
	if err != nil {
		t.Fatalf("ListEndpointsFiltered error: %v", err)
	}

	// Should return 4 endpoints (excluding .jpg and .png)
	if len(filtered) != 4 {
		t.Errorf("expected 4 filtered endpoints, got %d", len(filtered))
	}

	// Verify filtered endpoints don't include images
	for _, ep := range filtered {
		if ep.Path == "/image.jpg" || ep.Path == "/logo.png" {
			t.Errorf("filtered list should not include %s", ep.Path)
		}
	}
}

func TestMarkFilteredBatch(t *testing.T) {
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

	// Add endpoints
	urls := []string{
		"http://example.com/image.jpg",
		"http://example.com/photo.png",
		"http://example.com/page.html",
	}

	canonicals, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	// Mark first two as filtered
	toFilter := canonicals[:2]
	reasons := map[string]string{
		toFilter[0]: "extension:.jpg",
		toFilter[1]: "extension:.png",
	}

	err = ix.MarkFilteredBatch(ctx, toFilter, reasons)
	if err != nil {
		t.Fatalf("MarkFilteredBatch error: %v", err)
	}

	// Check status of filtered endpoints
	filteredEndpoints, err := ix.GetFilteredEndpoints(ctx, 100)
	if err != nil {
		t.Fatalf("GetFilteredEndpoints error: %v", err)
	}

	if len(filteredEndpoints) != 2 {
		t.Errorf("expected 2 filtered endpoints, got %d", len(filteredEndpoints))
	}

	// Verify they have filter reasons
	for _, ep := range filteredEndpoints {
		if ep.FilterReason == "" {
			t.Errorf("expected filter reason to be set for %s", ep.URL)
		}
	}
}

func TestUnfilterBatch(t *testing.T) {
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

	// Add endpoints
	urls := []string{
		"http://example.com/image.jpg",
		"http://example.com/page.html",
	}

	canonicals, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	// Mark first one as filtered
	toFilter := []string{canonicals[0]}
	reasons := map[string]string{
		toFilter[0]: "extension:.jpg",
	}

	err = ix.MarkFilteredBatch(ctx, toFilter, reasons)
	if err != nil {
		t.Fatalf("MarkFilteredBatch error: %v", err)
	}

	// Verify it's filtered
	filtered, _ := ix.GetFilteredEndpoints(ctx, 100)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered endpoint, got %d", len(filtered))
	}

	// Unfilter it
	err = ix.UnfilterBatch(ctx, toFilter)
	if err != nil {
		t.Fatalf("UnfilterBatch error: %v", err)
	}

	// Verify it's no longer filtered
	filtered, _ = ix.GetFilteredEndpoints(ctx, 100)
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered endpoints after unfilter, got %d", len(filtered))
	}

	// Verify it has pending status now
	pending, _ := ix.ListEndpoints(ctx, "pending", 100)
	foundPending := false
	for _, ep := range pending {
		if ep.CanonicalURL == canonicals[0] {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Error("expected unfiltered endpoint to be in pending status")
	}
}

func TestGetEndpointStats(t *testing.T) {
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

	// Add endpoints
	urls := []string{
		"http://example.com/page1.html",
		"http://example.com/page2.html",
		"http://example.com/image.jpg",
	}

	canonicals, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	// Mark one as pending, one as fetched, one as filtered
	if err := ix.MarkPending(ctx, canonicals[0]); err != nil {
		t.Fatalf("MarkPending error: %v", err)
	}
	if err := ix.MarkFetched(ctx, canonicals[1], "v1", time.Now()); err != nil {
		t.Fatalf("MarkFetched error: %v", err)
	}
	if err := ix.MarkFilteredBatch(ctx, []string{canonicals[2]}, map[string]string{canonicals[2]: "extension:.jpg"}); err != nil {
		t.Fatalf("MarkFilteredBatch error: %v", err)
	}

	// Get stats
	stats, err := ix.GetEndpointStats(ctx)
	if err != nil {
		t.Fatalf("GetEndpointStats error: %v", err)
	}

	if stats["pending"] != 1 {
		t.Errorf("expected 1 pending, got %d", stats["pending"])
	}
	if stats["fetched"] != 1 {
		t.Errorf("expected 1 fetched, got %d", stats["fetched"])
	}
	if stats["filtered"] != 1 {
		t.Errorf("expected 1 filtered, got %d", stats["filtered"])
	}
	if stats["total"] != 3 {
		t.Errorf("expected 3 total, got %d", stats["total"])
	}
}

func TestMarkPendingBatch_DoesNotOverrideFiltered(t *testing.T) {
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
	urls := []string{
		"http://example.com/filtered.jpg",
		"http://example.com/normal.html",
	}
	canonicals, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	if err := ix.MarkFilteredBatch(ctx, []string{canonicals[0]}, map[string]string{canonicals[0]: "extension:.jpg"}); err != nil {
		t.Fatalf("MarkFilteredBatch error: %v", err)
	}
	if err := ix.MarkPendingBatch(ctx, canonicals); err != nil {
		t.Fatalf("MarkPendingBatch error: %v", err)
	}

	filtered, err := ix.GetFilteredEndpoints(ctx, 100)
	if err != nil {
		t.Fatalf("GetFilteredEndpoints error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered endpoint, got %d", len(filtered))
	}
	if filtered[0].CanonicalURL != canonicals[0] {
		t.Fatalf("expected filtered endpoint %s, got %s", canonicals[0], filtered[0].CanonicalURL)
	}
}

func TestMarkFetchedBatch_DoesNotOverrideFiltered(t *testing.T) {
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
	urls := []string{
		"http://example.com/filtered.jpg",
		"http://example.com/normal.html",
	}
	canonicals, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	if err := ix.MarkFilteredBatch(ctx, []string{canonicals[0]}, map[string]string{canonicals[0]: "extension:.jpg"}); err != nil {
		t.Fatalf("MarkFilteredBatch error: %v", err)
	}
	if err := ix.MarkFetchedBatch(ctx, canonicals, "v1", time.Now()); err != nil {
		t.Fatalf("MarkFetchedBatch error: %v", err)
	}

	filtered, err := ix.GetFilteredEndpoints(ctx, 100)
	if err != nil {
		t.Fatalf("GetFilteredEndpoints error: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered endpoint, got %d", len(filtered))
	}
	if filtered[0].CanonicalURL != canonicals[0] {
		t.Fatalf("expected filtered endpoint %s, got %s", canonicals[0], filtered[0].CanonicalURL)
	}

	fetched, err := ix.ListEndpoints(ctx, "fetched", 100)
	if err != nil {
		t.Fatalf("ListEndpoints fetched error: %v", err)
	}
	foundNormalFetched := false
	for _, endpoint := range fetched {
		if endpoint.CanonicalURL == canonicals[1] {
			foundNormalFetched = true
		}
		if endpoint.CanonicalURL == canonicals[0] {
			t.Fatalf("filtered endpoint %s must not be marked fetched", canonicals[0])
		}
	}
	if !foundNormalFetched {
		t.Fatalf("expected non-filtered endpoint %s to be fetched", canonicals[1])
	}
}

func TestListEndpointsFiltered_PatternMatching(t *testing.T) {
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

	// Add various endpoints
	urls := []string{
		"http://example.com/api/users",
		"http://example.com/assets/js/app.js",
		"http://example.com/assets/css/style.css",
		"http://example.com/vendor/jquery.min.js",
		"http://example.com/page.html",
	}

	_, err := ix.AddEndpoints(ctx, urls, "spider")
	if err != nil {
		t.Fatalf("AddEndpoints error: %v", err)
	}

	// Create filter config that skips /assets/* pattern
	filterConfig := &filter.Config{
		SkipPatterns: []string{"*/assets/*"},
	}

	// List endpoints with filter
	filtered, err := ix.ListEndpointsFiltered(ctx, "*", 100, filterConfig)
	if err != nil {
		t.Fatalf("ListEndpointsFiltered error: %v", err)
	}

	// Should return 3 endpoints (excluding /assets/*)
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered endpoints, got %d", len(filtered))
	}

	// Verify no assets paths in result
	for _, ep := range filtered {
		if len(ep.Path) > 7 && ep.Path[:8] == "/assets/" {
			t.Errorf("filtered list should not include assets path: %s", ep.Path)
		}
	}
}
