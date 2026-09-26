-- name: DeleteBlockTopics :exec
DELETE FROM block_topics WHERE block_id = $1;

-- name: AddBlockTopic :exec
INSERT INTO block_topics (block_id, topic_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListBlockTopicIDs :many
SELECT bt.topic_id
FROM block_topics bt
JOIN topics t ON t.id = bt.topic_id AND t.archived_at IS NULL
WHERE bt.block_id = $1
ORDER BY t.sort_key;

-- name: CountTopicsInCourse :one
SELECT count(*) FROM topics
WHERE course_id = $1 AND id = ANY(sqlc.arg('topic_ids')::uuid[]) AND archived_at IS NULL;
