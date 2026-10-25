TRUNCATE TABLE users CASCADE;

DROP INDEX uq_users_username_active;

ALTER TABLE users
    DROP COLUMN username,
    DROP COLUMN password,
    ADD COLUMN google_subject VARCHAR(255) NOT NULL;

CREATE UNIQUE INDEX uq_users_google_subject_active ON users(google_subject) WHERE deleted_at IS NULL;

CREATE TABLE whitelisted_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    whitelisted BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX uq_whitelisted_users_email_active ON whitelisted_users(LOWER(email)) WHERE deleted_at IS NULL;

CREATE TABLE auth_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_auth_sessions_user_id ON auth_sessions(user_id);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions(expires_at);

CREATE TABLE oauth_authorization_transactions (
    state VARCHAR(128) PRIMARY KEY,
    nonce VARCHAR(128) NOT NULL,
    code_verifier VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_oauth_authorization_transactions_expires_at ON oauth_authorization_transactions(expires_at);
