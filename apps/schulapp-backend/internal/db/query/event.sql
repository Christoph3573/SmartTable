-- name: ListEvents :many
SELECT e.*
FROM events e
WHERE (e.start_time >= $1::timestamptz OR $1::timestamptz IS NULL)
  AND (e.end_time <= $2::timestamptz OR $2::timestamptz IS NULL)
  AND (e.class_id = $3::int OR $3::int IS NULL)
ORDER BY e.start_time;

-- name: CreateEvent :one
INSERT INTO events (title, start_time, end_time, all_day, type, class_id, creator_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateEvent :one
UPDATE events
SET title = COALESCE($1::text, title),
    start_time = COALESCE($2::timestamptz, start_time),
    end_time = COALESCE($3::timestamptz, end_time),
    all_day = COALESCE($4::bool, all_day),
    type = COALESCE($5::text, type)::text,
    class_id = COALESCE($6::int, class_id)
WHERE id = $7
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1;
