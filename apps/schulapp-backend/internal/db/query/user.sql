-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, role, active, created_at, updated_at
FROM users
WHERE email = $1 AND active = true;

-- name: GetUserByID :one
SELECT id, email, password_hash, first_name, last_name, role, active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUserName :exec
UPDATE users
SET first_name = COALESCE($1::text, first_name),
    last_name  = COALESCE($2::text, last_name),
    updated_at = NOW()
WHERE id = $3;

-- name: ListUsers :many
SELECT id, email, first_name, last_name, role, active, created_at
FROM users
ORDER BY last_name, first_name;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, first_name, last_name, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, first_name, last_name, role, active, created_at;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
