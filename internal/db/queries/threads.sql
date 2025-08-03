-- name: GetThread :one
SELECT * FROM threads
WHERE channel_id = ? AND thread_ts = ?
LIMIT 1;

-- name: CreateThread :one
INSERT INTO threads (
    channel_id, thread_ts, working_directory
) VALUES (
    ?, ?, ?
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

-- name: ListThreads :many
SELECT * FROM threads
ORDER BY updated_at DESC;

-- name: GetThreadByThreadTs :one
SELECT * FROM threads
WHERE thread_ts = ?
LIMIT 1;

-- name: ListThreadsPaginated :many
SELECT 
    t.*,
    s.initial_prompt AS first_session_prompt
FROM threads t
LEFT JOIN (
    SELECT 
        thread_id,
        initial_prompt,
        ROW_NUMBER() OVER (PARTITION BY thread_id ORDER BY started_at ASC) as rn
    FROM sessions
) s ON t.id = s.thread_id AND s.rn = 1
ORDER BY t.updated_at DESC
LIMIT ? OFFSET ?;