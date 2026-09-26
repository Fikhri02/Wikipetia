package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaHasSlice1Tables(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	conn, err := pgx.Connect(context.Background(), url)
	require.NoError(t, err)
	defer conn.Close(context.Background())

	want := []string{
		"users", "courses", "topics", "modules", "blocks", "block_topics",
		"questions", "enrolments", "block_progress", "topic_mastery", "events",
	}
	for _, table := range want {
		var exists bool
		err := conn.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema='public' AND table_name=$1)`, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", table)
	}
}

func TestSelfStudyEnrolmentIsUniquePerCourse(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	var userID, courseID string
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO users (name) VALUES ('u') RETURNING id`).Scan(&userID))
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO courses (owner_id, title) VALUES ($1,'c') RETURNING id`, userID).Scan(&courseID))

	_, err = tx.Exec(ctx,
		`INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1,$2,'self_study')`, userID, courseID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO enrolments (user_id, course_id, mode) VALUES ($1,$2,'self_study')`, userID, courseID)
	require.Error(t, err, "a second self-study enrolment in the same course must be rejected")
}
