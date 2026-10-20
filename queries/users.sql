-- name: GetUserByUsername :one
SELECT id, public_id, email, username, password, display_name, created_at, updated_at, deleted_at
FROM users
WHERE username = $1 AND deleted_at IS NULL;

-- name: GetUserByPublicID :one
SELECT id, public_id, email, username, password, display_name, created_at, updated_at, deleted_at
FROM users
WHERE public_id = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, public_id, email, username, password, display_name, created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (public_id, email, username, password, display_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, public_id, email, username, password, display_name, created_at, updated_at, deleted_at;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;
