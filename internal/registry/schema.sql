CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  INTEGER NOT NULL,
    meta        TEXT,
    dir_name    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS websites (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL,
    slug         TEXT NOT NULL,
    origin       TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER,
    config       TEXT,
    dir_name     TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS filter_rules (
    id         TEXT PRIMARY KEY,
    website_id TEXT NOT NULL,
    rule_type  TEXT NOT NULL,  -- "extension", "pattern", "status_code"
    rule_value TEXT NOT NULL,  -- ".jpg", "*/media/*", "404"
    priority   INTEGER DEFAULT 0, -- Higher = evaluated first (patterns > extensions)
    enabled    INTEGER DEFAULT 1, -- SQLite boolean: 0=false, 1=true
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (website_id) REFERENCES websites(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_filter_rules_website ON filter_rules(website_id);
CREATE INDEX IF NOT EXISTS idx_filter_rules_enabled ON filter_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_filter_rules_priority ON filter_rules(priority DESC);
