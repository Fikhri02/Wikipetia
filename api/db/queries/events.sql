-- name: InsertEvent :one
INSERT INTO events (user_id, enrolment_id, kind, payload)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListUnconsumedEvents :many
SELECT id, occurred_at, user_id, enrolment_id, kind, payload
FROM events
WHERE consumed_at IS NULL
ORDER BY id
LIMIT $1;

-- name: MarkEventConsumed :execrows
UPDATE events SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL;

-- name: CountEventsByKind :one
SELECT count(*) FROM events WHERE enrolment_id = $1 AND kind = $2;
