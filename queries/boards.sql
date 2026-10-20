-- name: CreateBoard :one
INSERT INTO boards (public_id, user_id, title, description)
VALUES ($1, $2, $3, $4)
RETURNING id, public_id, user_id, title, description, created_at, updated_at, deleted_at;

-- name: GetBoardByPublicID :one
SELECT id, public_id, user_id, title, description, created_at, updated_at, deleted_at
FROM boards
WHERE public_id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: GetBoardByPublicIDAnyUser :one
SELECT id, public_id, user_id, title, description, created_at, updated_at, deleted_at
FROM boards
WHERE public_id = $1 AND deleted_at IS NULL;

-- name: ListBoardsByUserID :many
SELECT id, public_id, user_id, title, description, created_at, updated_at, deleted_at
FROM boards
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: UpdateBoard :one
UPDATE boards
SET title = $1, description = $2, updated_at = NOW()
WHERE public_id = $3 AND user_id = $4 AND deleted_at IS NULL
RETURNING id, public_id, user_id, title, description, created_at, updated_at, deleted_at;

-- name: SoftDeleteBoard :one
UPDATE boards
SET deleted_at = NOW(), updated_at = NOW()
WHERE public_id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING id;

-- name: CascadeSoftDeleteTasksByBoardID :exec
UPDATE tasks
SET deleted_at = NOW(), updated_at = NOW()
WHERE board_id = $1 AND deleted_at IS NULL;
