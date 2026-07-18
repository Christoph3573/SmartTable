-- name: ListHomeworkByClass :many
SELECT h.*
FROM homework h
WHERE h.class_id = $1
ORDER BY h.due_date;

-- name: CreateHomework :one
INSERT INTO homework (title, description, due_date, class_id, subject_id, teacher_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateHomework :one
UPDATE homework
SET title = COALESCE($1::text, title),
    description = COALESCE($2::text, description),
    due_date = COALESCE($3::date, due_date),
    subject_id = COALESCE($4::int, subject_id)
WHERE id = $5
RETURNING *;

-- name: DeleteHomework :exec
DELETE FROM homework WHERE id = $1;

-- name: ListSubmissionsByHomework :many
SELECT hs.*
FROM homework_submissions hs
WHERE hs.homework_id = $1
ORDER BY hs.student_id;

-- name: CreateSubmission :exec
INSERT INTO homework_submissions (homework_id, student_id, status)
VALUES ($1, $2, 'submitted')
ON CONFLICT (homework_id, student_id) DO UPDATE SET status = 'submitted', submitted_at = NOW();

-- name: GradeSubmission :one
UPDATE homework_submissions
SET grade = $1, status = 'graded', graded_at = NOW()
WHERE id = $2
RETURNING *;
