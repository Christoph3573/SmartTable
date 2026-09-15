-- name: ListLessonsByClass :many
SELECT l.*
FROM lessons l
WHERE l.class_id = $1
ORDER BY l.day_of_week, l.period;

-- name: CreateLesson :one
INSERT INTO lessons (class_id, subject_id, teacher_id, day_of_week, period, room)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateLesson :one
UPDATE lessons
SET subject_id  = COALESCE($1::int, subject_id),
    teacher_id  = COALESCE($2::int, teacher_id),
    day_of_week = COALESCE($3::int, day_of_week),
    period      = COALESCE($4::int, period),
    room        = COALESCE($5::text, room)
WHERE id = $6
RETURNING *;

-- name: DeleteLesson :exec
DELETE FROM lessons WHERE id = $1;
