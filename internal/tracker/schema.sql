-- SQLite schema for moku tracker
-- This schema defines the data model for version control of website snapshots

-- Meta table: configuration and schema version
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Insert schema version
INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_version', '1');

-- Snapshots: captured web content at a point in time
CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    file_path TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    headers TEXT
);

CREATE INDEX IF NOT EXISTS idx_snapshots_created_at ON snapshots(created_at);
CREATE INDEX IF NOT EXISTS idx_snapshots_url ON snapshots(url);

-- Versions: commits/history entries
CREATE TABLE IF NOT EXISTS versions (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    snapshot_id TEXT NOT NULL,
    message TEXT NOT NULL,
    author TEXT,
    timestamp INTEGER NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES versions(id),
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id)
);

CREATE INDEX IF NOT EXISTS idx_versions_timestamp ON versions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_versions_parent_id ON versions(parent_id);
CREATE INDEX IF NOT EXISTS idx_versions_snapshot_id ON versions(snapshot_id);

-- Version files: many-to-many relationship between versions and file blobs
CREATE TABLE IF NOT EXISTS version_files (
    version_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    blob_id TEXT NOT NULL,
    size INTEGER NOT NULL,
    PRIMARY KEY (version_id, file_path),
    FOREIGN KEY (version_id) REFERENCES versions(id)
);

CREATE INDEX IF NOT EXISTS idx_version_files_blob_id ON version_files(blob_id);

-- Diffs: precomputed diffs between versions
CREATE TABLE IF NOT EXISTS diffs (
    id TEXT PRIMARY KEY,
    base_version_id TEXT,
    head_version_id TEXT NOT NULL,
    diff_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (base_version_id) REFERENCES versions(id),
    FOREIGN KEY (head_version_id) REFERENCES versions(id)
);

CREATE INDEX IF NOT EXISTS idx_diffs_base_head ON diffs(base_version_id, head_version_id);
CREATE INDEX IF NOT EXISTS idx_diffs_head ON diffs(head_version_id);

-- Scans: tracking for analyzer scan results
CREATE TABLE IF NOT EXISTS scans (
    id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL,
    scan_data TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (version_id) REFERENCES versions(id)
);

CREATE INDEX IF NOT EXISTS idx_scans_version_id ON scans(version_id);
CREATE INDEX IF NOT EXISTS idx_scans_created_at ON scans(created_at);

-- Sequence: sequence numbers for ordering operations
CREATE TABLE IF NOT EXISTS seq (
    name TEXT PRIMARY KEY,
    value INTEGER NOT NULL DEFAULT 0
);
