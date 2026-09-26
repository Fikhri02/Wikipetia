-- name: CreateQuestion :one
INSERT INTO questions (block_id, topic_id, kind, prompt, answer, sort_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetQuestionByID :one
SELECT * FROM questions WHERE id = $1 AND archived_at IS NULL;

-- name: ListQuestionsByBlock :many
SELECT * FROM questions
WHERE block_id = $1 AND archived_at IS NULL
ORDER BY sort_key;

-- name: UpdateQuestion :one
UPDATE questions SET
    topic_id = sqlc.narg('topic_id'),
    prompt   = COALESCE(sqlc.narg('prompt'), prompt),
    answer   = COALESCE(sqlc.narg('answer'), answer)
WHERE id = sqlc.arg('id') AND archived_at IS NULL
RETURNING *;

-- name: ArchiveQuestion :execrows
UPDATE questions SET archived_at = now() WHERE id = $1 AND archived_at IS NULL;
