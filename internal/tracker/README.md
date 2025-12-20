# Tracker Component Design

## Overview

The tracker component provides version control for website snapshots, similar to git but optimized for web content. It stores snapshots with headers, manages versions, computes body and header diffs, and provides checkout semantics for historical content. The tracker is designed to be integrated with the fetcher through an adapter layer (integration is optional and can be implemented by the consuming application).

## Architecture

### Directory Layout

```
siteDir/
  .moku/
    moku.db          # SQLite database (metadata)
    blobs/           # Content-addressed blob storage
      ab/
        cd1234...    # sha256 hex filename (first 2 chars as subdirectory)
    HEAD             # Current version ID (text file)
  example/           # Working-tree directory for a page path (e.g., "/example")
    .page_body         # Raw HTML content (convenience file)
    .page_headers.json # Normalized headers as JSON (convenience file)
```

The `.moku` directory is the authoritative store. The working tree (files outside `.moku`) contains convenience files that are written after each commit and checkout:
- `.page_body`: Raw body content from the snapshot
- `.page_headers.json`: Normalized HTTP headers in human-readable JSON format

These working-tree files can be regenerated from the blob store at any time.

### Data Model

#### Tables

**meta**: Configuration and schema version
- `key` TEXT PRIMARY KEY
- `value` TEXT

**snapshots**: Captured web content at a point in time
- `id` TEXT PRIMARY KEY (UUID)
- `version_id` TEXT (foreign key to versions.id, NOT NULL)
- `status_code` INTEGER
- `url` TEXT (source URL)
- `file_path` TEXT (relative path in working tree, e.g., "/example")
- `blob_id` TEXT (sha256 hex pointing into blobstore)
- `created_at` INTEGER (Unix timestamp)
- `headers` TEXT (JSON-encoded normalized HTTP headers)

**versions**: Commits/history entries
- `id` TEXT PRIMARY KEY (UUID)
- `parent_id` TEXT (parent version, NULL for initial)
- `message` TEXT (commit message)
- `author` TEXT (optional)
- `timestamp` INTEGER (Unix timestamp)

**diffs**: Precomputed diffs between versions
- `id` TEXT PRIMARY KEY (UUID)
- `base_version_id` TEXT
- `head_version_id` TEXT
- `diff_json` TEXT (JSON serialized combined diff with body_diff and headers_diff)
- `created_at` INTEGER

**version_scores**: Stored scoring results for versions
- `id` TEXT PRIMARY KEY (UUID)
- `version_id` TEXT
- `scoring_version` TEXT
- `score` REAL
- `normalized` INTEGER
- `confidence` REAL
- `score_json` TEXT
- `created_at` INTEGER

**version_evidence_locations**: Evidence location storage used for attribution
- columns for evidence/location indices, selector/xpath, file_path, ranges, etc.

**diff_attributions**: Contribution rows linking evidence to diff chunks
- contribution weights and percentages per-evidence/location

### Content-Addressed Storage

Blobs are stored under `.moku/blobs/` using SHA-256 hash as the filename. The first two characters of the hash form a subdirectory to avoid filesystem limitations with too many files in one directory.

Example:
- Content hash: `abcd1234...`
- Storage path: `.moku/blobs/ab/cd1234...`

This ensures:
1. Deduplication: identical content stored once
2. Integrity: filename is the content hash
3. Efficient storage: shared content across versions

## Core Operations

### Commit Flow

#### Single Commit

1. **Store blobs**: Write snapshot body to blob storage
   - Compute SHA-256 hash of content
   - Store at `.moku/blobs/{hash[0:2]}/{hash}`
   - Use atomic write (tmp file → fsync → rename)

2. **Begin transaction**: Start SQLite transaction

3. **Extract and normalize headers**: Parse headers from snapshot metadata, normalize (lowercase, sort, redact sensitive)

4. **Insert version**: Create version record linking to parent

5. **Insert snapshot**: Create snapshot record with version_id, URL, file_path, timestamp, normalized headers JSON

6. **Compute diffs**: Calculate combined diff (body + headers) from parent version (if exists)
   - Extract text content from both versions
   - Run diff algorithm (diffmatchpatch for body or a line-based diff)
   - Compute header diff (added, removed, changed, redacted)
   - Store combined diff JSON in diffs table with base_version_id and head_version_id

7. **Commit transaction**: Persist all metadata

8. **Write working-tree files**: Write `.page_body` and `.page_headers.json` using AtomicWriteFile

9. **Write HEAD**: Update `.moku/HEAD` with new version ID

#### Batch Commit

The `CommitBatch` method efficiently commits multiple snapshots in a single transaction:

1. **Store all blobs**: Write all snapshot bodies to blob storage in parallel
2. **Extract and normalize headers** for each snapshot
3. **Begin transaction**
4. **Insert version record**
5. **Insert all snapshots** with version_id
6. **Compute diffs** for each file against parent version (if exists)
7. **Commit transaction**
8. **Write working-tree files** for all snapshots
9. **Write HEAD**

This is more efficient than individual commits when fetching multiple pages, as it uses a single transaction and reduces database overhead.

### Diff Algorithm

**Text diffing** (initial implementation):
- Extract HTML content from both versions
- Compute line-based unified diff
- Return as JSON with chunks: `{type: "added"|"removed"|"modified", content: "...", path: "..."}`

**Header normalization**:
- Normalize HTTP headers before storage and comparison
- Lowercase header names for case-insensitive handling
- Trim whitespace from values
- Sort multi-value headers (except order-sensitive ones like Set-Cookie)
- Redact sensitive headers (Authorization, Cookie, API keys, etc.)
- Store normalized form as JSON in snapshots.headers column

**Header diffing**:
- Compute structured diff between normalized headers
- Track added headers (present in head, not in base)
- Track removed headers (present in base, not in head)
- Track changed headers (different values between base and head)
- Track redacted headers (sensitive headers present in either version)
- Diff structure: `{added: map, removed: map, changed: map, redacted: []string}`

**DOM-aware diffing** (future):
- Parse HTML into DOM tree
- Compute structural diff (node insertions, deletions, moves)
- More precise than text diff for HTML

### Checkout Semantics

**Checkout version N**:
1. Read snapshots for version N (WHERE version_id = N)
2. For each file_path → blob_id:
   - Read blob from `.moku/blobs/{hash[0:2]}/{hash}`
   - Write to working tree at file_path using AtomicWriteFile
3. Update `.moku/HEAD` to version N

**Safety**:
- Never modify blobs (immutable)
- Always use atomic writes to working tree
- Maintain HEAD consistency

### Garbage Collection (GC)

**Unreachable blob detection**:
1. Mark phase: Walk all versions, collect all referenced blob_ids
2. Sweep phase: Delete blobs not in referenced set

**Version retention policy**:
- Keep last N versions (configurable)
- Keep versions with specific tags/labels
- Prune old diffs older than retention period

**Manual GC trigger**: `tracker.GC(ctx, retentionPolicy)`

### Canonicalization Rules

**File paths**:
- Always use forward slashes
- Relative to siteDir
- Sanitize to prevent directory traversal (reject `..`, absolute paths)

**Content normalization** (for diffing):
- Line ending normalization (CRLF → LF)
- Trailing whitespace handling (configurable)
- Header canonicalization (strip volatile headers)

**URL normalization**:
- Lowercase scheme and host
- Remove default ports (80 for http, 443 for https)
- Normalize path (remove `./`, resolve `../`)

## Implementation Status

### Current Implementation

**Completed**:
- SQLite-backed tracker (SQLiteTracker) with commit, batch commit, list, diff, checkout, close
- Blob storage using content-addressed files (`internal/tracker/blobstore`)
- Normalized header storage and redaction logic
- Combined diff JSON persisted in `diffs`
- Scoring and attribution via `internal/tracker/score` and SQLiteTracker.ScoreAndAttributeVersion
- Tests covering commit/get/list/diff/checkout and header normalization

**Deprecated/Removed**:
- In-memory tracker (NewInMemoryTracker) and related tests
- FSStore helpers replaced by blobstore package

**Planned / Next steps**:
1. Enhance diff algorithm and DOM-aware mapping
2. GC of unreachable blobs and old diffs
3. Pagination and filtering for List
4. Performance tuning for large pages

## Testing Strategy

**Unit tests**:
- AtomicWriteFile: verify fsync, rename, permissions
- FSStore: Put/Get/Exists/Delete operations
- Path sanitization: reject malicious paths
- Schema application: verify all tables created
- Commit flow: mock blob storage, verify transaction

**Integration tests**:
- End-to-end commit → diff → checkout cycle
- Multiple versions with branching
- GC removes unreachable blobs
- Concurrent commits (transaction isolation)

**Performance tests**:
- Large snapshot commits (>10MB HTML)
- Diff computation on similar large documents
- Checkout performance with many files
- Blob storage scalability

## Security Considerations

1. **Path traversal prevention**: Sanitize all file paths, reject `..` and absolute paths
2. **Atomic writes**: Use fsync before rename to prevent corruption
3. **Transaction isolation**: SQLite WAL mode for concurrent reads
4. **Content integrity**: SHA-256 hashing ensures tamper detection
5. **Input validation**: Validate UUIDs, timestamps, sizes
6. **Resource limits**: Cap snapshot size, number of versions, GC frequency

## Configuration

```go
type Config struct {
    StoragePath             string         // Root directory; .moku lives under this path
    ProjectID               string         // Optional project identifier to enforce in DB meta
    ForceProjectID          bool           // Overwrite existing project_id when true
    RedactSensitiveHeaders  *bool          // If nil, defaults to true; controls header redaction
}
```

## Diff JSON Format

The `diff_json` column in the diffs table contains a combined diff with both body and header changes:

```json
{
  "body_diff": {
    "base_id": "version-uuid-1",
    "head_id": "version-uuid-2",
    "chunks": [
      {
        "type": "removed",
        "content": "Old Title"
      },
      {
        "type": "added",
        "content": "New Title"
      }
    ]
  },
  "headers_diff": {
    "added": {
      "x-custom-header": ["value1", "value2"]
    },
    "removed": {
      "x-deprecated-header": ["old-value"]
    },
    "changed": {
      "content-type": {
        "from": ["text/html"],
        "to": ["application/json"]
      },
      "cache-control": {
        "from": ["no-cache"],
        "to": ["max-age=3600", "public"]
      }
    },
    "redacted": ["authorization", "cookie"]
  }
}
```

### Header Normalization Rules

1. **Case normalization**: Header names are lowercased (`Content-Type` → `content-type`)
2. **Value trimming**: Leading/trailing whitespace is removed from values
3. **Sorting**: Multi-value headers are sorted alphabetically (except order-sensitive ones)
4. **Order preservation**: Headers like `Set-Cookie` preserve original order
5. **Redaction**: Sensitive headers are replaced with `["[REDACTED]"]` if redaction is enabled

### Sensitive Headers (Redacted)

The following headers are considered sensitive and are redacted in storage and diffs:
- `authorization`
- `cookie`
- `set-cookie`
- `proxy-authorization`
- `www-authenticate`
- `proxy-authenticate`
- `x-api-key`
- `x-auth-token`

## Fetcher Integration (note)

Fetcher integration is not enabled by default in this repository. The tracker is intentionally implemented as an independent component that can be integrated with a fetcher via an adapter in the consuming application. This repository provides the tracker APIs and the recommended pattern for integration (fetcher builds Snapshot instances and calls tracker.Commit or tracker.CommitBatch). Integrating the fetcher is left to the application maintainers and is not assumed to be present in the current codebase.

## Example Usage (Tracker-only)

```go
// Initialize tracker
logger := logging.NewStdoutLogger("tracker")
t, err := tracker.NewSQLiteTracker(logger, nil, &tracker.Config{StoragePath: "/path/to/site"})
if err != nil {
    log.Fatal(err)
}
defer t.Close()

// Commit a snapshot with headers (direct tracker API)
headers := map[string][]string{
    "Content-Type": {"text/html; charset=utf-8"},
    "Cache-Control": {"no-cache", "no-store"},
}
headersJSON, _ := json.Marshal(headers)

snapshot := &tracker.Snapshot{
    URL:  "https://example.com",
    Body: []byte("<html>...</html>"),
    Headers: headers,
}
cr, err := t.Commit(ctx, snapshot, "Initial commit", "user@example.com")

// Batch commit multiple snapshots (more efficient)
snapshots := []*tracker.Snapshot{snapshot1, snapshot2, snapshot3}
cr, err := t.CommitBatch(ctx, snapshots, "Batch commit", "user@example.com")

// Get diff between versions (returns body diff for backward compatibility)
diff, err := t.Diff(ctx, crs[0].Version.ID, cr.Version.ID)

// For full combined diff including headers, query diffs table directly
// SELECT diff_json FROM diffs WHERE base_version_id = ? AND head_version_id = ?

// Checkout a specific version (writes working-tree files)
err = t.Checkout(ctx, cr.Version.ID)

// List recent versions
versions, err := t.List(ctx, 10)
```

### Working-Tree Files

After a commit or checkout, the tracker writes convenience files to the working tree:

- `siteDir/example/.page_body` - Raw HTML content
- `siteDir/example/.page_headers.json` - Normalized headers in pretty-printed JSON

These files are for human readability and convenience. The authoritative data is always in `.moku/` (blobs and database).

Example `.page_headers.json`:
```json
{
  "cache-control": [
    "max-age=3600",
    "public"
  ],
  "content-type": [
    "text/html; charset=utf-8"
  ],
  "server": [
    "nginx/1.20.0"
  ]
}
```

Note that sensitive headers (authorization, cookie, etc.) are replaced with `["[REDACTED]"]` if redaction is enabled in the tracker configuration.

## References

- Git internals: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain
- Content-addressed storage: https://en.wikipedia.org/wiki/Content-addressable_storage
- SQLite best practices: https://www.sqlite.org/pragma.html
- Unified diff format: https://en.wikipedia.org/wiki/Diff#Unified_format
