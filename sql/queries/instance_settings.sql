-- name: GetSigningSecret :one
SELECT token_secret FROM instance_settings WHERE id = 1;

-- name: InitializeSigningSecret :one
UPDATE instance_settings
SET token_secret = CASE WHEN token_secret = '' THEN sqlc.arg(secret) ELSE token_secret END
WHERE id = 1
RETURNING token_secret;

-- name: GetAdmin :one
SELECT username, password_hash FROM users WHERE id = 1;

-- name: CreateAdmin :execrows
INSERT INTO users (id, username, password_hash) VALUES (1, ?, ?)
ON CONFLICT(id) DO NOTHING;

-- name: UpdateAdminPassword :execrows
UPDATE users SET password_hash = sqlc.arg(new_hash)
WHERE id = 1 AND password_hash = sqlc.arg(current_hash);
