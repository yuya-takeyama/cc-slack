-- name: GetThread :one
SELECT * FROM threads
WHERE channel_id = ? AND thread_ts = ?
LIMIT 1;

-- name: CreateThread :one
INSERT INTO threads (
    channel_id, thread_ts
) VALUES (
    ?, ?
)
RETURNING *;

-- name: UpdateThreadTimestamp :exec
UPDATE threads
SET updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetThreadByID :one
SELECT * FROM threads
WHERE id = ?
LIMIT 1;