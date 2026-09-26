package events_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func TestAppendAndReadBack(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|learner", "learner")
	require.NoError(t, err)

	require.NoError(t, events.Append(ctx, q, events.Event{
		UserID: userID,
		Kind:   events.KindBlockCompleted,
		Payload: map[string]any{
			"block_id": uuid.New().String(),
		},
	}))

	records, err := events.ListUnconsumed(ctx, q, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, events.KindBlockCompleted, records[0].Kind)
	assert.Contains(t, string(records[0].Payload), "block_id")
}

func TestAppendRejectsUnknownKind(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|learner", "learner")
	require.NoError(t, err)

	err = events.Append(ctx, q, events.Event{
		UserID: userID, Kind: "pet.fed", Payload: map[string]any{},
	})
	require.Error(t, err, "only kinds the learning side emits are allowed")
}

func TestEventsAreRolledBackWithTheirTransaction(t *testing.T) {
	// The harness rolls back, so this event must not survive to the next test.
	ctx, q := pgtest.Begin(t)

	userID, err := user.Store{}.EnsureBySubject(ctx, q, "dev|rollback", "rollback")
	require.NoError(t, err)

	require.NoError(t, events.Append(ctx, q, events.Event{
		UserID: userID, Kind: events.KindTopicMastered,
		Payload: map[string]any{"topic_id": uuid.New().String()},
	}))
}

func TestPreviousTestsEventIsGone(t *testing.T) {
	ctx, q := pgtest.Begin(t)

	records, err := events.ListUnconsumed(ctx, q, 100)
	require.NoError(t, err)
	assert.Empty(t, records,
		"an event written in a rolled-back transaction must not persist — this is the outbox guarantee")
}
