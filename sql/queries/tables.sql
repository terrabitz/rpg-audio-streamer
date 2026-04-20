-- name: GetTables :many
SELECT * FROM tables ORDER BY created_at DESC;

-- name: GetTableByInviteCode :one
SELECT * FROM tables WHERE invite_code = @invite_code;