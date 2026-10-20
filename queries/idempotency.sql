-- name: GetIdempotencyKey :one
SELECT idempotency_key, request_hash, task_id, original_task, created_at
FROM idempotency_keys
WHERE idempotency_key = $1;

-- name: CreateIdempotencyKey :exec
INSERT INTO idempotency_keys (idempotency_key, request_hash, task_id, original_task, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteExpiredIdempotencyKeys :exec
DELETE FROM idempotency_keys
WHERE created_at <= $1;
