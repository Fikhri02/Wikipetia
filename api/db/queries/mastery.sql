-- name: GetTopicMastery :one
SELECT * FROM topic_mastery WHERE enrolment_id = $1 AND topic_id = $2;

-- name: UpsertTopicMastery :one
INSERT INTO topic_mastery (enrolment_id, topic_id, strength, evidence_count, last_evidence_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
ON CONFLICT (enrolment_id, topic_id) DO UPDATE SET
    strength         = EXCLUDED.strength,
    evidence_count   = EXCLUDED.evidence_count,
    last_evidence_at = now(),
    updated_at       = now()
RETURNING *;

-- name: ListTopicMasteryByEnrolment :many
SELECT tm.topic_id, tm.strength, tm.evidence_count, tm.last_evidence_at
FROM topic_mastery tm
JOIN topics t ON t.id = tm.topic_id AND t.archived_at IS NULL
WHERE tm.enrolment_id = $1
ORDER BY t.sort_key;
