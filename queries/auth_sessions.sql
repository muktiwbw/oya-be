-- name: CreateAuthSession :one
INSERT INTO auth_sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at, revoked_at;

-- name: GetAuthSessionByTokenHash :one
SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
FROM auth_sessions
WHERE token_hash = $1;

-- name: RotateAuthSession :one
WITH revoked AS (
    UPDATE auth_sessions
    SET revoked_at = NOW()
    WHERE auth_sessions.token_hash = sqlc.arg(previous_token_hash)
      AND auth_sessions.revoked_at IS NULL
      AND auth_sessions.expires_at > sqlc.arg(now)
    RETURNING user_id
)
INSERT INTO auth_sessions (user_id, token_hash, expires_at)
SELECT user_id, sqlc.arg(new_token_hash), sqlc.arg(new_expires_at)
FROM revoked
RETURNING id, user_id, token_hash, expires_at, created_at, revoked_at;

-- name: RevokeAuthSessionByTokenHash :exec
UPDATE auth_sessions
SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredAuthSessions :exec
DELETE FROM auth_sessions WHERE expires_at <= $1;
