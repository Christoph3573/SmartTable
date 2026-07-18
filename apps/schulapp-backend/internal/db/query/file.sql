-- name: ListFoldersByClass :many
SELECT * FROM file_folders
WHERE class_id = $1
ORDER BY name;

-- name: CreateFolder :one
INSERT INTO file_folders (name, parent_id, class_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteFolder :exec
DELETE FROM file_folders WHERE id = $1;

-- name: ListFilesByClass :many
SELECT * FROM files
WHERE class_id = $1
  AND (folder_id = $2::int OR $2::int IS NULL)
ORDER BY created_at DESC;

-- name: CreateFile :one
INSERT INTO files (name, path, size, mime_type, uploader_id, class_id, folder_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetFileByID :one
SELECT * FROM files WHERE id = $1;

-- name: DeleteFile :exec
DELETE FROM files WHERE id = $1;
