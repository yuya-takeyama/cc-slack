-- name: GetWorktree :one
SELECT * FROM worktrees
WHERE id = ? LIMIT 1;

-- name: GetWorktreeByThreadID :one
SELECT * FROM worktrees
WHERE thread_id = ? LIMIT 1;

-- name: ListActiveWorktrees :many
SELECT * FROM worktrees
WHERE status = 'active'
ORDER BY created_at DESC;

-- name: ListWorktreesByRepository :many
SELECT * FROM worktrees
WHERE repository_id = ?
ORDER BY created_at DESC;

-- name: CreateWorktree :one
INSERT INTO worktrees (
    repository_id, thread_id, path, 
    base_branch, current_branch, status
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateWorktreeStatus :exec
UPDATE worktrees
SET 
    status = ?,
    deleted_at = CASE WHEN ? = 'deleted' THEN CURRENT_TIMESTAMP ELSE deleted_at END
WHERE id = ?;

-- name: UpdateWorktreeBranch :exec
UPDATE worktrees
SET current_branch = ?
WHERE id = ?;

-- name: ListOldWorktrees :many
SELECT * FROM worktrees
WHERE status = 'active'
AND created_at < datetime('now', ?)
ORDER BY created_at;

-- name: DeleteWorktree :exec
DELETE FROM worktrees
WHERE id = ?;