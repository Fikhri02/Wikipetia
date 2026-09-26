-- name: CreateCourse :one
INSERT INTO courses (owner_id, title, description, visibility, pet_enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCourseByID :one
SELECT * FROM courses WHERE id = $1 AND archived_at IS NULL;

-- name: ListCoursesByOwner :many
SELECT * FROM courses
WHERE owner_id = $1 AND archived_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateCourse :one
UPDATE courses SET
    title       = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility  = COALESCE(sqlc.narg('visibility'), visibility),
    pet_enabled = COALESCE(sqlc.narg('pet_enabled'), pet_enabled)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: ArchiveCourse :execrows
UPDATE courses SET archived_at = now()
WHERE id = $1 AND archived_at IS NULL;
