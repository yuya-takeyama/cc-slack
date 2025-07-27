-- name: GetWorkingDirectory :one
SELECT * FROM working_directories
WHERE channel_id = ? AND name = ?
LIMIT 1;

-- name: GetWorkingDirectoriesByChannel :many
SELECT * FROM working_directories
WHERE channel_id = ?
ORDER BY name;

-- name: CreateWorkingDirectory :one
INSERT INTO working_directories (
    channel_id, name, path
) VALUES (
    ?, ?, ?
) RETURNING *;

-- name: UpdateWorkingDirectory :one
UPDATE working_directories
SET path = ?, updated_at = CURRENT_TIMESTAMP
WHERE channel_id = ? AND name = ?
RETURNING *;

-- name: DeleteWorkingDirectory :exec
DELETE FROM working_directories
WHERE channel_id = ? AND name = ?;