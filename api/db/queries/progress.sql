-- name: GetBlockProgress :one
SELECT * FROM block_progress WHERE enrolment_id = $1 AND block_id = $2;

-- name: UpsertBlockProgress :one
INSERT INTO block_progress (enrolment_id, block_id, status, score, attempts, completed_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (enrolment_id, block_id) DO UPDATE SET
    status       = EXCLUDED.status,
    score        = EXCLUDED.score,
    attempts     = EXCLUDED.attempts,
    completed_at = COALESCE(block_progress.completed_at, EXCLUDED.completed_at),
    updated_at   = now()
RETURNING *;

-- name: GetBlockForProgress :one
SELECT b.id, b.type, b.completion, m.course_id
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
WHERE b.id = $1 AND b.archived_at IS NULL;
