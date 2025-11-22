# Tracker Component Design

## Overview

The tracker component provides version control for website snapshots, similar to git but optimized for web content. It stores snapshots, manages versions, computes diffs, and provides checkout semantics for historical content.

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
```

The `.moku` directory is the authoritative store. The working tree (files outside `.moku`) is derived from the current HEAD version and can be regenerated from the blob store.

### Data Model

#### Tables

**meta**: Configuration and schema version
- `key` TEXT PRIMARY KEY
- `value` TEXT

**snapshots**: Captured web content at a point in time
- `id` TEXT PRIMARY KEY (UUID)
- `url` TEXT (source URL)
- `file_path` TEXT (relative path in working tree, e.g., "index.html")
- `created_at` INTEGER (Unix timestamp)

**versions**: Commits/history entries
- `id` TEXT PRIMARY KEY (UUID)
- `parent_id` TEXT (parent version, NULL for initial)
- `snapshot_id` TEXT REFERENCES snapshots(id)
- `message` TEXT (commit message)
- `author` TEXT (optional)
- `timestamp` INTEGER (Unix timestamp)

**version_files**: Many-to-many relationship between versions and file blobs
- `version_id` TEXT REFERENCES versions(id)
- `file_path` TEXT (relative path)
- `blob_id` TEXT (sha256 hex of content)
- `size` INTEGER (file size in bytes)
- PRIMARY KEY (version_id, file_path)

**diffs**: Precomputed diffs between versions
- `id` TEXT PRIMARY KEY (UUID)
- `base_version_id` TEXT
- `head_version_id` TEXT
- `diff_json` TEXT (JSON serialized diff chunks)
- `created_at` INTEGER

**scans**: Tracking for analyzer scan results
- `id` TEXT PRIMARY KEY (UUID)
- `version_id` TEXT REFERENCES versions(id)
- `scan_data` TEXT (JSON)
- `created_at` INTEGER

**seq**: Sequence numbers for ordering operations
- `name` TEXT PRIMARY KEY
- `value` INTEGER

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

1. **Store blobs**: Write snapshot body and any related files to blob storage
   - Compute SHA-256 hash of content
   - Store at `.moku/blobs/{hash[0:2]}/{hash}`
   - Use atomic write (tmp file → fsync → rename)

2. **Begin transaction**: Start SQLite transaction

3. **Insert snapshot**: Create snapshot record with ID, URL, file_path, timestamp

4. **Insert version**: Create version record linking to snapshot and parent

5. **Insert version_files**: Create entries mapping file_path → blob_id for this version

6. **Compute diffs**: Calculate diff from parent version (if exists)
   - Extract text content from both versions
   - Run diff algorithm (placeholder: unified diff or JSON-based)
   - Store in diffs table with base_version_id and head_version_id

7. **Commit transaction**: Persist all metadata

8. **Update working tree**: Write files to working directory using AtomicWriteFile

9. **Write HEAD**: Update `.moku/HEAD` with new version ID

### Diff Algorithm

**Text diffing** (initial implementation):
- Extract HTML content from both versions
- Compute line-based unified diff
- Return as JSON with chunks: `{type: "added"|"removed"|"modified", content: "...", path: "..."}`

**Header canonicalization** (TODO):
- Normalize HTTP headers before comparison
- Strip volatile headers (Date, Set-Cookie, etc.)
- Sort remaining headers
- Store canonical form in metadata

**DOM-aware diffing** (future):
- Parse HTML into DOM tree
- Compute structural diff (node insertions, deletions, moves)
- More precise than text diff for HTML

### Checkout Semantics

**Checkout version N**:
1. Read version_files for version N
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

### Current Implementation (Skeleton)

**Completed**:
- Interface definition (`interfaces.Tracker`)
- Model types (`model.Snapshot`, `model.Version`, `model.DiffResult`)
- In-memory tracker scaffold (returns ErrNotImplemented)

**This PR adds**:
- SQLite schema (`schema.sql`)
- FSStore for blob storage (`store_fs.go`)
- SQLiteTracker skeleton (`sqlite_tracker.go`)
- Helper functions (`fs_helpers.go`, `helpers.go`)
- Commit flow skeleton with placeholders

**TODO (Next Steps)**:
1. Implement actual diff algorithm (text-based unified diff)
2. Add header canonicalization logic
3. Implement Checkout method fully
4. Implement List with pagination
5. Add GC implementation
6. Add tests for commit flow
7. Add tests for diff computation
8. Add benchmarks for large snapshots
9. Consider compression for blob storage
10. Add support for tags/labels on versions

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
    SiteDir       string        // Root directory containing .moku
    MaxSnapshotMB int           // Max snapshot size (default: 100MB)
    MaxVersions   int           // Version retention (default: 100)
    GCInterval    time.Duration // Auto-GC frequency (default: off)
    DBPath        string        // Override DB path (default: .moku/moku.db)
    BlobsPath     string        // Override blobs path (default: .moku/blobs)
}
```

## Example Usage

```go
// Initialize tracker
logger := logging.NewStdoutLogger("tracker")
tracker, err := tracker.NewSQLiteTracker("/path/to/site", logger)
if err != nil {
    log.Fatal(err)
}
defer tracker.Close()

// Commit a snapshot
snapshot := &model.Snapshot{
    URL:  "https://example.com",
    Body: []byte("<html>...</html>"),
    Meta: map[string]string{"Content-Type": "text/html"},
}
version, err := tracker.Commit(ctx, snapshot, "Initial commit", "user@example.com")

// Get diff between versions
diff, err := tracker.Diff(ctx, parentID, currentID)

// Checkout a specific version (future)
err = tracker.Checkout(ctx, versionID)

// List recent versions
versions, err := tracker.List(ctx, 10)
```

## References

- Git internals: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain
- Content-addressed storage: https://en.wikipedia.org/wiki/Content-addressable_storage
- SQLite best practices: https://www.sqlite.org/pragma.html
- Unified diff format: https://en.wikipedia.org/wiki/Diff#Unified_format
