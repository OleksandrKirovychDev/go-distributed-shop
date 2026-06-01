-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (id, user_id, token_hash, issued_at, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, issued_at, expires_at, revoked_at, replaced_by
FROM refresh_tokens
WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = $2, replaced_by = $3
WHERE id = $1;

-- name: RevokeAllRefreshTokensForUser :exec
UPDATE refresh_tokens
SET revoked_at = $2
WHERE user_id = $1 AND revoked_at IS NULL;
