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
    status_code INTEGER NOT NULL,
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

-- Version files: many-to-many relationship between versions and file blobs (Not necessary for now but maybe useful later)
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

-- Adds per-version score storage.
CREATE TABLE IF NOT EXISTS version_scores (
  id TEXT PRIMARY KEY,           -- uuid
  version_id TEXT NOT NULL,      -- REFERENCES versions(id)
  scoring_version TEXT NOT NULL, -- heuristics scoring version used
  score REAL NOT NULL,           -- numeric score
  normalized INTEGER,            -- optional normalized integer score
  confidence REAL,               -- assessor confidence
  score_json TEXT NOT NULL,      -- JSON-serialized ScoreResult (full evidence, matched rules)
  created_at INTEGER NOT NULL,
  UNIQUE(version_id, scoring_version),
  FOREIGN KEY (version_id) REFERENCES versions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_version_scores_version_id ON version_scores(version_id);

-- Flattened evidence-location rows per version (one row per evidence location).
-- If an EvidenceItem has no locations, a single row is stored with location_index = -1.
CREATE TABLE IF NOT EXISTS version_evidence_locations (
  id TEXT PRIMARY KEY,               -- uuid
  version_id TEXT NOT NULL,          -- references versions(id)
  evidence_id TEXT NOT NULL,         -- evidence item id (from ScoreResult)
  evidence_index INTEGER NOT NULL,   -- index of evidence in ScoreResult.Evidence
  location_index INTEGER NOT NULL,   -- index within evidence.Locations (0..N-1), -1 for "global"/none
  evidence_key TEXT,                 -- evidence key (ev.Key)
  selector TEXT,
  xpath TEXT,
  node_id TEXT,
  file_path TEXT,
  byte_start INTEGER,
  byte_end INTEGER,
  line_start INTEGER,
  line_end INTEGER,
  loc_confidence REAL,               -- per-location confidence if provided
  evidence_json TEXT NOT NULL,       -- full EvidenceItem JSON (for audit)
  location_json TEXT,                -- raw Location JSON (for audit)
  scoring_version TEXT,              -- scoring version that produced this evidence (optional convenience)
  created_at INTEGER NOT NULL,
  FOREIGN KEY(version_id) REFERENCES versions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_vel_version_id ON version_evidence_locations(version_id);
CREATE INDEX IF NOT EXISTS idx_vel_evidence_id ON version_evidence_locations(evidence_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vel_unique_triplet ON version_evidence_locations(version_id, evidence_id, location_index);

-- Attribution rows that link diff chunks to specific per-version evidence locations.
-- Each attribution may optionally reference version_evidence_locations(id) for an authoritative link.
CREATE TABLE IF NOT EXISTS diff_attributions (
  id TEXT PRIMARY KEY,                       -- uuid
  diff_id TEXT NOT NULL,                     -- references diffs(id)
  head_version_id TEXT NOT NULL,             -- denormalized: the head version for which this attribution was computed
  version_evidence_location_id TEXT,         -- fk -> version_evidence_locations(id) (nullable)
  evidence_id TEXT NOT NULL,                 -- evidence item id (redundant for ease of queries)
  evidence_location_index INTEGER,           -- index within evidence.Locations or -1 for global
  chunk_index INTEGER NOT NULL,              -- index of chunk within combined body diff (or -1 for "global")
  evidence_key TEXT,                         -- ruleID|evidenceKey (optional human-friendly key)
  evidence_json TEXT,                        -- JSON of the evidence item (optional duplicate for audit)
  location_json TEXT,                        -- JSON of the matched location (optional duplicate for audit)
  scoring_version TEXT NOT NULL,             -- scoring version that produced this attribution
  contribution REAL NOT NULL,                -- numeric contribution weight (un-normalized)
  contribution_pct REAL NOT NULL,            -- normalized percent (0..100)
  note TEXT,                                 -- optional human note
  created_at INTEGER NOT NULL,
  FOREIGN KEY (diff_id) REFERENCES diffs(id) ON DELETE CASCADE,
  FOREIGN KEY (head_version_id) REFERENCES versions(id) ON DELETE CASCADE,
  FOREIGN KEY (version_evidence_location_id) REFERENCES version_evidence_locations(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_diff_attributions_diff_id ON diff_attributions(diff_id);
CREATE INDEX IF NOT EXISTS idx_diff_attributions_head_version ON diff_attributions(head_version_id);
CREATE INDEX IF NOT EXISTS idx_diff_attributions_vel_id ON diff_attributions(version_evidence_location_id);
CREATE INDEX IF NOT EXISTS idx_diff_attributions_scoring_version ON diff_attributions(scoring_version);
