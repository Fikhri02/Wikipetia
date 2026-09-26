-- name: CreateModule :one
INSERT INTO modules (course_id, title, sort_key) VALUES ($1, $2, $3) RETURNING *;

-- name: ListModulesByCourse :many
SELECT * FROM modules
WHERE course_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: GetModuleByID :one
SELECT * FROM modules WHERE id = $1 AND archived_at IS NULL;

-- name: UpdateModuleTitle :one
UPDATE modules SET title = $2 WHERE id = $1 AND archived_at IS NULL RETURNING *;

-- name: UpdateModuleSortKey :execrows
UPDATE modules SET sort_key = $2 WHERE id = $1 AND archived_at IS NULL;

-- name: ArchiveModule :execrows
UPDATE modules SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
