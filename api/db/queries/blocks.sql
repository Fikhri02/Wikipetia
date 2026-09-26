-- name: CreateBlock :one
INSERT INTO blocks (module_id, title, type, sort_key, content, completion)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetBlockByID :one
SELECT * FROM blocks WHERE id = $1 AND archived_at IS NULL;

-- name: ListBlocksByModule :many
SELECT * FROM blocks
WHERE module_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: UpdateBlock :one
UPDATE blocks SET
    title      = COALESCE(sqlc.narg('title'), title),
    content    = COALESCE(sqlc.narg('content'), content),
    completion = COALESCE(sqlc.narg('completion'), completion)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: UpdateBlockSortKey :execrows
UPDATE blocks SET sort_key = $2 WHERE id = $1 AND archived_at IS NULL;

-- name: ArchiveBlock :execrows
UPDATE blocks SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;

-- name: GetBlockCourseID :one
SELECT m.course_id
FROM blocks b
JOIN modules m ON m.id = b.module_id
WHERE b.id = $1 AND b.archived_at IS NULL AND m.archived_at IS NULL;
