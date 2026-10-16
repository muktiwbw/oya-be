CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('todo','doing','done')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_tasks_public_id ON tasks(public_id);
CREATE INDEX idx_tasks_created_at ON tasks(created_at DESC,public_id ASC);
CREATE INDEX idx_tasks_status_created_at ON tasks(status,created_at DESC,public_id ASC);
CREATE TABLE idempotency_keys (
    idempotency_key TEXT PRIMARY KEY NOT NULL,
    request_hash TEXT NOT NULL,
    task_id INTEGER NOT NULL REFERENCES tasks(id),
    original_task TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys(created_at);
