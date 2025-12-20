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
