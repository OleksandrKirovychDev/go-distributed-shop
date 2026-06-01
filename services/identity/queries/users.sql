-- name: InsertUser :exec
INSERT INTO users (id, email, password_hash, roles, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetUserByID :one
SELECT id, email, password_hash, roles, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, roles, created_at, updated_at
FROM users
WHERE email = $1;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = $2, updated_at = $3
WHERE id = $1;
