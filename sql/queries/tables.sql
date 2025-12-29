-- name: GetTables :many
SELECT * FROM tables ORDER BY created_at DESC;