-- name: CreateOAuthTransaction :one
INSERT INTO oauth_authorization_transactions (state, nonce, code_verifier, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING state, nonce, code_verifier, expires_at, created_at, consumed_at;

-- name: ConsumeOAuthTransaction :one
UPDATE oauth_authorization_transactions
SET consumed_at = NOW()
WHERE state = $1 AND consumed_at IS NULL AND expires_at > $2
RETURNING state, nonce, code_verifier, expires_at, created_at, consumed_at;

-- name: DeleteExpiredOAuthTransactions :exec
DELETE FROM oauth_authorization_transactions WHERE expires_at <= $1;
