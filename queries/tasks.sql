-- name: CreateTask :one
INSERT INTO tasks (public_id, board_id, title, description, status, version, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, public_id, board_id, title, description, status, version, created_at, updated_at, deleted_at;

-- name: GetTaskByPublicID :one
SELECT t.id, t.public_id, t.board_id, b.public_id AS board_public_id, t.title, t.description, t.status, t.version, t.created_at, t.updated_at, t.deleted_at
FROM tasks t
JOIN boards b ON b.id = t.board_id
WHERE t.public_id = $1 AND t.deleted_at IS NULL AND b.deleted_at IS NULL;

-- name: GetTaskByPublicIDAndUserID :one
SELECT t.id, t.public_id, t.board_id, b.public_id AS board_public_id, t.title, t.description, t.status, t.version, t.created_at, t.updated_at, t.deleted_at
FROM tasks t
JOIN boards b ON b.id = t.board_id
WHERE t.public_id = $1 AND b.user_id = $2 AND t.deleted_at IS NULL AND b.deleted_at IS NULL;

-- name: UpdateTaskByPublicID :one
UPDATE tasks
SET title = $1, description = $2, version = version + 1, updated_at = $3
WHERE public_id = $4 AND version = $5 AND deleted_at IS NULL
RETURNING id, public_id, board_id, title, description, status, version, created_at, updated_at, deleted_at;

-- name: UpdateTaskStatusByPublicID :one
UPDATE tasks
SET status = $1, version = version + 1, updated_at = $2
WHERE public_id = $3 AND version = $4 AND deleted_at IS NULL
RETURNING id, public_id, board_id, title, description, status, version, created_at, updated_at, deleted_at;

-- name: SoftDeleteTaskByPublicID :exec
UPDATE tasks
SET deleted_at = NOW(), updated_at = NOW()
WHERE public_id = $1 AND deleted_at IS NULL;
