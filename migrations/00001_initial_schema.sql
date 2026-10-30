-- +goose Up

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(255) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX idx_users_public_id ON users(public_id);
CREATE UNIQUE INDEX uq_users_email_active ON users(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_username_active ON users(username) WHERE deleted_at IS NULL;

CREATE TABLE boards (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX idx_boards_public_id ON boards(public_id);
CREATE INDEX idx_boards_user_id ON boards(user_id) WHERE deleted_at IS NULL;

CREATE TABLE tasks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL,
    board_id BIGINT NOT NULL REFERENCES boards(id),
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL CHECK (status IN ('todo', 'doing', 'done')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX idx_tasks_public_id ON tasks(public_id);
CREATE INDEX idx_tasks_board_id ON tasks(board_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_board_created_at ON tasks(board_id, created_at DESC, public_id ASC) WHERE deleted_at IS NULL;

CREATE TABLE idempotency_keys (
    idempotency_key VARCHAR(128) PRIMARY KEY NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    task_id BIGINT NOT NULL REFERENCES tasks(id),
    original_task TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- +goose Down

DROP TABLE idempotency_keys;
DROP TABLE tasks;
DROP TABLE boards;
DROP TABLE users;
