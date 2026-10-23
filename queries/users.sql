-- name: GetUserByGoogleSubject :one
SELECT id, public_id, email, google_subject, display_name, created_at, updated_at, deleted_at
FROM users
WHERE google_subject = $1 AND deleted_at IS NULL;

-- name: GetUserByID :one
SELECT id, public_id, email, google_subject, display_name, created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateGoogleUser :one
INSERT INTO users (public_id, email, google_subject, display_name)
VALUES ($1, $2, $3, $4)
RETURNING id, public_id, email, google_subject, display_name, created_at, updated_at, deleted_at;

-- name: IsUserWhitelisted :one
SELECT EXISTS (
    SELECT 1
    FROM users u
    JOIN whitelisted_users w ON LOWER(w.email) = LOWER(u.email)
    WHERE u.id = $1 AND u.deleted_at IS NULL AND w.deleted_at IS NULL AND w.whitelisted = TRUE
);

-- name: IsEmailWhitelisted :one
SELECT EXISTS (
    SELECT 1 FROM whitelisted_users
    WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL AND whitelisted = TRUE
);
