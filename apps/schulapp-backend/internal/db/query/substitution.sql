-- name: ListSubstitutions :many
SELECT s.*
FROM substitutions s
WHERE (s.date >= $1::date OR $1::date IS NULL)
  AND (s.date <= $2::date OR $2::date IS NULL)
  AND (s.class_id = $3::int OR $3::int IS NULL)
ORDER BY s.date, s.period;

-- name: CreateSubstitution :one
INSERT INTO substitutions (date, period, class_id, subject_id, original_teacher_id, sub_teacher_id, room, type, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateSubstitution :one
UPDATE substitutions
SET date = COALESCE($1::date, date),
    period = COALESCE($2::int, period),
    class_id = COALESCE($3::int, class_id),
    subject_id = COALESCE($4::int, subject_id),
    original_teacher_id = COALESCE($5::int, original_teacher_id),
    sub_teacher_id = COALESCE($6::int, sub_teacher_id),
    room = COALESCE($7::text, room),
    type = COALESCE($8::text, type)::text,
    note = COALESCE($9::text, note)
WHERE id = $10
RETURNING *;

-- name: DeleteSubstitution :exec
DELETE FROM substitutions WHERE id = $1;
