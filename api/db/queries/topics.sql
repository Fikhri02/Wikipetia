-- name: CreateTopic :one
INSERT INTO topics (course_id, title, sort_key) VALUES ($1, $2, $3) RETURNING *;

-- name: ListTopicsByCourse :many
SELECT * FROM topics
WHERE course_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: GetTopicByID :one
SELECT * FROM topics WHERE id = $1 AND archived_at IS NULL;

-- name: UpdateTopicTitle :one
UPDATE topics SET title = $2 WHERE id = $1 AND archived_at IS NULL RETURNING *;

-- name: ArchiveTopic :execrows
UPDATE topics SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
