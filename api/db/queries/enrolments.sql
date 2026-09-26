-- name: CreateEnrolment :one
INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1, $2, $3) RETURNING *;

-- name: GetEnrolmentByID :one
SELECT * FROM enrolments WHERE id = $1;

-- name: ListEnrolmentsByUser :many
SELECT e.*, c.title AS course_title
FROM enrolments e
JOIN courses c ON c.id = e.course_id AND c.archived_at IS NULL
WHERE e.user_id = $1
ORDER BY e.started_at DESC;
