-- Встроенная схема (fallback, если миграции goose недоступны).
-- Должна быть идентична migrations/0001_init.sql.

CREATE TABLE IF NOT EXISTS sessions (
    id              TEXT PRIMARY KEY,
    user_key        TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    expires_at      INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    ip              TEXT,
    user_agent      TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_key ON sessions(user_key);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_key        TEXT NOT NULL,
    project_id      TEXT,
    registry_id     TEXT,
    action          TEXT NOT NULL,
    target          TEXT,
    ip              TEXT,
    user_agent      TEXT,
    status          TEXT NOT NULL,
    error_message   TEXT,
    meta            TEXT,
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_key, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action, created_at);

CREATE TABLE IF NOT EXISTS share_links (
    short_id        TEXT PRIMARY KEY,
    user_key        TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    registry_id     TEXT NOT NULL,
    file_path       TEXT NOT NULL,
    password_hash   TEXT,
    max_downloads   INTEGER,
    downloads       INTEGER NOT NULL DEFAULT 0,
    expires_at      INTEGER,
    revoked         INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_share_user ON share_links(user_key, created_at);
CREATE INDEX IF NOT EXISTS idx_share_expires ON share_links(expires_at);

CREATE TABLE IF NOT EXISTS folder_markers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_key        TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    registry_id     TEXT NOT NULL,
    full_path       TEXT NOT NULL,
    created_by      TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    UNIQUE (project_id, registry_id, full_path)
);
CREATE INDEX IF NOT EXISTS idx_folder_markers_lookup
    ON folder_markers(project_id, registry_id, full_path);

CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    key             TEXT PRIMARY KEY,
    tokens          REAL NOT NULL,
    updated_at      INTEGER NOT NULL
);
