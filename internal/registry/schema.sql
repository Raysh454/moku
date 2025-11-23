-- registry migrations: create projects and websites tables
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,           -- UUID
  slug TEXT UNIQUE NOT NULL,     -- human short id for CLI
  name TEXT NOT NULL,
  description TEXT,
  created_at INTEGER NOT NULL,
  meta TEXT
);

CREATE TABLE IF NOT EXISTS websites (
  id TEXT PRIMARY KEY,           -- UUID
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  slug TEXT NOT NULL,            -- short id within project; unique per project
  name TEXT,
  origin TEXT NOT NULL,          -- canonical origin/url root
  storage_path TEXT NOT NULL,    -- absolute path to siteDir (or relative to StorageRoot)
  created_at INTEGER NOT NULL,
  last_seen_at INTEGER,
  config TEXT,
  UNIQUE(project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_websites_project ON websites(project_id);
CREATE INDEX IF NOT EXISTS idx_websites_origin ON websites(origin);
