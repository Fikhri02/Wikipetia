-- name: GetUserBySubject :one
SELECT * FROM users WHERE auth_subject = $1;

-- name: CreateUser :one
INSERT INTO users (auth_subject, name) VALUES ($1, $2) RETURNING *;
