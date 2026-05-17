-- +goose Up
-- =============================================================================
-- AR Drive — начальная схема
-- =============================================================================

-- Серверные сессии. user_key = sha256(client_id + project_id)
CREATE TABLE IF NOT EXISTS sessions (
    id              TEXT PRIMARY KEY,           -- random UUID
    user_key        TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    created_at      INTEGER NOT NULL,           -- unix sec
    expires_at      INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    ip              TEXT,
    user_agent      TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_key ON sessions(user_key);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Аудит-журнал
CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_key        TEXT NOT NULL,
    project_id      TEXT,
    registry_id     TEXT,
    action          TEXT NOT NULL,              -- upload, download, delete, edit, create_folder, ...
    target          TEXT,                       -- путь файла/папки
    ip              TEXT,
    user_agent      TEXT,
    status          TEXT NOT NULL,              -- success | error
    error_message   TEXT,
    meta            TEXT,                       -- JSON
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_key, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action, created_at);

-- Шеринг-ссылки
CREATE TABLE IF NOT EXISTS share_links (
    short_id        TEXT PRIMARY KEY,           -- 10-12 знаков base62
    user_key        TEXT NOT NULL,              -- кто создал
    project_id      TEXT NOT NULL,
    registry_id     TEXT NOT NULL,
    file_path       TEXT NOT NULL,
    password_hash   TEXT,                       -- bcrypt, NULL = без пароля
    max_downloads   INTEGER,                    -- NULL = без лимита
    downloads       INTEGER NOT NULL DEFAULT 0,
    expires_at      INTEGER,                    -- unix sec, NULL = бессрочно
    revoked         INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_share_user ON share_links(user_key, created_at);
CREATE INDEX IF NOT EXISTS idx_share_expires ON share_links(expires_at);

-- Виртуальные маркеры папок (workaround для Generic-реестров)
CREATE TABLE IF NOT EXISTS folder_markers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_key        TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    registry_id     TEXT NOT NULL,
    full_path       TEXT NOT NULL,              -- "/dir/subdir/"
    created_by      TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    UNIQUE (project_id, registry_id, full_path)
);
CREATE INDEX IF NOT EXISTS idx_folder_markers_lookup
    ON folder_markers(project_id, registry_id, full_path);

-- Fallback токен-бакеты для rate-limit
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    key             TEXT PRIMARY KEY,
    tokens          REAL NOT NULL,
    updated_at      INTEGER NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TABLE IF EXISTS folder_markers;
DROP TABLE IF EXISTS share_links;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS sessions;
