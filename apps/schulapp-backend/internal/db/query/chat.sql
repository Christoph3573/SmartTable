-- name: ListUserChannels :many
SELECT c.*
FROM chat_channels c
JOIN chat_members cm ON cm.channel_id = c.id
WHERE cm.user_id = $1
ORDER BY c.created_at DESC;

-- name: CreateChannel :one
INSERT INTO chat_channels (name, type, class_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AddChannelMember :exec
INSERT INTO chat_members (channel_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetChannelMessages :many
SELECT m.*
FROM messages m
WHERE m.channel_id = $1
  AND (m.id < $2::int OR $2::int IS NULL)
ORDER BY m.id DESC
LIMIT $3;

-- name: SendMessage :one
INSERT INTO messages (channel_id, sender_id, content, file_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateLastRead :exec
INSERT INTO chat_members (channel_id, user_id, last_read_at)
VALUES ($1, $2, NOW())
ON CONFLICT (channel_id, user_id) DO UPDATE SET last_read_at = NOW();
