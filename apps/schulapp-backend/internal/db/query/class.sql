-- name: ListClasses :many
SELECT id, name, school_year, created_at
FROM classes
ORDER BY name;

-- name: GetClassByID :one
SELECT id, name, school_year, created_at
FROM classes
WHERE id = $1;

-- name: CreateClass :one
INSERT INTO classes (name, school_year)
VALUES ($1, $2)
RETURNING id, name, school_year, created_at;

-- name: UpdateClass :one
UPDATE classes
SET name = COALESCE($1::text, name),
    school_year = COALESCE($2::text, school_year)
WHERE id = $3
RETURNING id, name, school_year, created_at;

-- name: DeleteClass :exec
DELETE FROM classes WHERE id = $1;

-- name: ListClassMembers :many
SELECT u.id, u.email, u.first_name, u.last_name, u.role
FROM class_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.class_id = $1
ORDER BY u.last_name, u.first_name;

-- name: AddClassMember :exec
INSERT INTO class_members (class_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveClassMember :exec
DELETE FROM class_members
WHERE class_id = $1 AND user_id = $2;

-- name: ListClassTeachers :many
SELECT u.id, u.email, u.first_name, u.last_name, ct.is_home_teacher
FROM class_teachers ct
JOIN users u ON u.id = ct.user_id
WHERE ct.class_id = $1
ORDER BY u.last_name, u.first_name;

-- name: AddClassTeacher :exec
INSERT INTO class_teachers (class_id, user_id, is_home_teacher)
VALUES ($1, $2, $3)
ON CONFLICT (class_id, user_id) DO UPDATE SET is_home_teacher = $3;

-- name: RemoveClassTeacher :exec
DELETE FROM class_teachers
WHERE class_id = $1 AND user_id = $2;
