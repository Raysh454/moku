-- SQLite schema for moku tracker
-- This schema defines the data model for version control of website snapshots

-- Meta table: configuration and schema version
CREATE TABLE IF NOT EXISTS meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Insert schema version
INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_version', '2');

-- Snapshots: captured web content at a point in time
-- Each snapshot represents a single URL fetch and corresponds to exactly one file

-- Currently, each commit creates a new snapshot regardless of whether the file is changed, we can fix this by:
--  adding: fetched_at, shows when content was last fetched, is updated if content is not changed and no new snapshot needs to be created
--  adding: content_hash, sha256(status_code + headers + body), to detect changed content, only create new snapshot if changed
-- but this all seems useless as we currently do not have a way to detect meaningful changes deterministically.
-- Headers and page data change constantly, so we would have a lot of snapshots eitherways

CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    status_code INTEGER NOT NULL,
    url TEXT NOT NULL,
    file_path TEXT NOT NULL,
    blob_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    headers TEXT
);

CREATE INDEX IF NOT EXISTS idx_snapshots_created_at ON snapshots(created_at);
CREATE INDEX IF NOT EXISTS idx_snapshots_url ON snapshots(url);
CREATE INDEX IF NOT EXISTS idx_snapshots_blob_id ON snapshots(blob_id);

-- Versions: commits/history entries
-- A version represents a commit that may reference many snapshots
CREATE TABLE IF NOT EXISTS versions (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    message TEXT NOT NULL,
    author TEXT,
    timestamp INTEGER NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES versions(id)
);

CREATE INDEX IF NOT EXISTS idx_versions_timestamp ON versions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_versions_parent_id ON versions(parent_id);

-- Version snapshots: many-to-many relationship between versions and snapshots
CREATE TABLE IF NOT EXISTS version_snapshots (
    version_id TEXT NOT NULL,
    snapshot_id TEXT NOT NULL,
    PRIMARY KEY (version_id, snapshot_id),
    FOREIGN KEY (version_id) REFERENCES versions(id) ON DELETE CASCADE,
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_version_snapshots_version_id ON version_snapshots(version_id);
CREATE INDEX IF NOT EXISTS idx_version_snapshots_snapshot_id ON version_snapshots(snapshot_id);

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

-- Creates per-site endpoints table for persisted discovered URL metadata
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

CREATE TABLE IF NOT EXISTS score_results (
    id            TEXT PRIMARY KEY,
    version_id    TEXT NOT NULL,

    score         REAL    NOT NULL,  -- 0.0 .. 1.0
    normalized    INTEGER NOT NULL,  -- 0 .. 100
    confidence    REAL    NOT NULL,  -- 0.0 .. 1.0
    scoring_version   TEXT    NOT NULL,  -- scoring rules version
    created_at     INTEGER NOT NULL,

    score_json    TEXT,              -- JSON
    matched_rules TEXT,              -- JSON
    meta          TEXT,              -- JSON
    raw_features  TEXT,              -- JSON

    FOREIGN KEY (version_id)
        REFERENCES versions(id)
        ON DELETE CASCADE
);

-- Enforce exactly one score per version
CREATE UNIQUE INDEX IF NOT EXISTS idx_score_results_version
ON score_results(version_id);

CREATE TABLE IF NOT EXISTS evidence_items (
    id               TEXT PRIMARY KEY,
    score_result_id  TEXT NOT NULL,

    evidence_uid     TEXT,           -- stable ID inside ScoreResult
    item_key              TEXT NOT NULL,
    rule_id          TEXT,
    severity         TEXT NOT NULL,
    description      TEXT NOT NULL,
    value            TEXT,  -- JSON
    contribution     REAL,  -- score contribution from this evidence

    FOREIGN KEY (score_result_id)
        REFERENCES score_results(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evidence_items_score
ON evidence_items(score_result_id);

CREATE INDEX IF NOT EXISTS idx_evidence_items_severity
ON evidence_items(severity);

CREATE TABLE IF NOT EXISTS evidence_locations (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    evidence_item_id  TEXT NOT NULL,

    snapshot_id       TEXT NOT NULL,  -- exact file version
    location_type     TEXT,            -- "css", "regex", "header", "cookie", "xpath"
    css_selector      TEXT,
    xpath             TEXT,
    regex_pattern     TEXT,
    file_path         TEXT,

    byte_start        INTEGER,
    byte_end          INTEGER,
    line_start        INTEGER,
    line_end          INTEGER,
    line              INTEGER,
    column            INTEGER,

    header_name       TEXT,            -- for header-based evidence
    cookie_name       TEXT,            -- for cookie-based evidence

    note              TEXT,

    FOREIGN KEY (evidence_item_id)
        REFERENCES evidence_items(id)
        ON DELETE CASCADE,

    FOREIGN KEY (snapshot_id)
        REFERENCES snapshots(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evidence_locations_item
ON evidence_locations(evidence_item_id);

CREATE INDEX IF NOT EXISTS idx_evidence_locations_snapshot
ON evidence_locations(snapshot_id);
