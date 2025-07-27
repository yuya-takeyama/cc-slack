-- name: GetSession :one
SELECT * FROM sessions
WHERE session_id = ?
LIMIT 1;

-- name: GetLatestSessionByThread :one
SELECT s.*
FROM sessions s
WHERE s.thread_id = ?
  AND s.status = 'completed'
ORDER BY s.ended_at DESC
LIMIT 1;

-- name: GetActiveSessionByThread :one
SELECT s.*
FROM sessions s
WHERE s.thread_id = ?
  AND s.status = 'active'
ORDER BY s.started_at DESC
LIMIT 1;

-- name: CreateSession :one
INSERT INTO sessions (
    thread_id, session_id, model
) VALUES (
    ?, ?, ?
)
RETURNING *;

-- name: UpdateSessionStatus :exec
UPDATE sessions
SET status = ?,
    ended_at = CURRENT_TIMESTAMP,
    total_cost_usd = ?,
    input_tokens = ?,
    output_tokens = ?,
    duration_ms = ?,
    num_turns = ?
WHERE session_id = ?;

-- name: UpdateSessionEndTime :exec
UPDATE sessions
SET status = ?,
    ended_at = CURRENT_TIMESTAMP
WHERE session_id = ?;

-- name: ListActiveSessions :many
SELECT * FROM sessions
WHERE status = 'active'
ORDER BY started_at DESC;

-- name: CountActiveSessionsByThread :one
SELECT COUNT(*) as count
FROM sessions
WHERE thread_id = ? AND status = 'active';

-- name: UpdateSessionID :exec
UPDATE sessions
SET session_id = ?
WHERE session_id = ?;

-- name: UpdateSessionOnTimeout :exec
UPDATE sessions
SET status = ?,
    ended_at = CURRENT_TIMESTAMP
WHERE session_id = ?;

-- name: UpdateSessionOnError :exec
UPDATE sessions
SET status = ?,
    ended_at = CURRENT_TIMESTAMP
WHERE session_id = ?;

-- name: UpdateSessionOnComplete :exec
UPDATE sessions
SET status = ?,
    ended_at = ?,
    total_cost_usd = ?,
    input_tokens = ?,
    output_tokens = ?,
    num_turns = ?,
    model = ?
WHERE session_id = ?;

-- name: GetThreadBySlackIDs :one
SELECT t.*
FROM threads t
WHERE t.channel_id = ? AND t.thread_ts = ?
LIMIT 1;

-- name: UpdateSessionModel :exec
UPDATE sessions
SET model = ?
WHERE session_id = ?;