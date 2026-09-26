-- name: OutlineBlocks :many
SELECT
    b.id,
    b.module_id,
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
    )::uuid[] AS topic_ids,
    (SELECT count(*) FROM questions qq
     WHERE qq.block_id = b.id AND qq.archived_at IS NULL) AS question_count
FROM blocks b
JOIN modules m ON m.id = b.module_id AND m.archived_at IS NULL
WHERE m.course_id = $1 AND b.archived_at IS NULL
ORDER BY m.sort_key, b.sort_key;
