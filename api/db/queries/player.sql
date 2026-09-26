-- name: PlayerCourseHeader :one
SELECT c.id AS course_id, c.title AS course_title, c.pet_enabled, e.started_at, e.mode
FROM enrolments e
JOIN courses c ON c.id = e.course_id
WHERE e.id = $1;

-- name: PlayerBlocks :many
SELECT
    b.id,
    b.module_id,
    m.title      AS module_title,
    m.sort_key   AS module_sort_key,
    b.title,
    b.type,
    b.sort_key,
    b.content,
    b.completion,
    COALESCE(
        (SELECT array_agg(bt.topic_id ORDER BY t.sort_key)
         FROM block_topics bt
         JOIN topics t ON t.id = bt.topic_id AND t.archived_at IS NULL
         WHERE bt.block_id = b.id),
        '{}'
    )::uuid[]   AS topic_ids,
    bp.status,
    bp.score,
    COALESCE(bp.attempts, 0)::integer AS attempts
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
JOIN enrolments e ON e.course_id = m.course_id
LEFT JOIN block_progress bp ON bp.block_id = b.id AND bp.enrolment_id = e.id
WHERE e.id = $1 AND b.archived_at IS NULL
ORDER BY m.sort_key, b.sort_key;

-- name: PlayerQuestions :many
SELECT q.id, q.block_id, q.kind, q.prompt, q.sort_key
FROM questions q
JOIN blocks b ON b.id = q.block_id AND b.archived_at IS NULL
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
JOIN enrolments e ON e.course_id = m.course_id
WHERE e.id = $1 AND q.archived_at IS NULL
ORDER BY q.block_id, q.sort_key;

-- name: PlayerTopics :many
SELECT
    t.id AS topic_id,
    t.title,
    COALESCE(tm.strength, 0)::double precision AS strength,
    COALESCE(tm.evidence_count, 0)::integer    AS evidence_count,
    tm.last_evidence_at
FROM enrolments e
JOIN topics t ON t.course_id = e.course_id AND t.archived_at IS NULL
LEFT JOIN topic_mastery tm ON tm.topic_id = t.id AND tm.enrolment_id = e.id
WHERE e.id = $1
ORDER BY t.sort_key;
