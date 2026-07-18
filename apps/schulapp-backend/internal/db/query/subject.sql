-- name: ListSubjects :many
SELECT id, name, short FROM subjects ORDER BY name;

-- name: GetSubjectByID :one
SELECT id, name, short FROM subjects WHERE id = $1;

-- name: CreateSubject :one
INSERT INTO subjects (name, short) VALUES ($1, $2)
RETURNING id, name, short;
