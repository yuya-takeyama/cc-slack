-- name: GetRepository :one
SELECT * FROM repositories
WHERE id = ? LIMIT 1;

-- name: GetRepositoryByName :one
SELECT * FROM repositories
WHERE name = ? LIMIT 1;

-- name: GetRepositoryByChannelID :one
SELECT * FROM repositories
WHERE slack_channel_id = ? LIMIT 1;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY name;

-- name: ListRepositoriesByChannelID :many
SELECT * FROM repositories
WHERE slack_channel_id = ?
ORDER BY name;

-- name: CreateRepository :one
INSERT INTO repositories (
    name, path, default_branch, 
    slack_channel_id, slack_username, 
    slack_icon_emoji, slack_icon_url
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateRepository :one
UPDATE repositories
SET
    name = ?,
    path = ?,
    default_branch = ?,
    slack_channel_id = ?,
    slack_username = ?,
    slack_icon_emoji = ?,
    slack_icon_url = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteRepository :exec
DELETE FROM repositories
WHERE id = ?;