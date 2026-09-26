package course_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestCreateTopicsKeepInsertionOrder(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	for _, title := range []string{"Atoms", "Bonding", "Stoichiometry"} {
		_, err := svc.CreateTopic(ctx, owner, courseID, title)
		require.NoError(t, err)
	}

	list, err := svc.ListTopics(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "Atoms", list[0].Title)
	assert.Equal(t, "Bonding", list[1].Title)
	assert.Equal(t, "Stoichiometry", list[2].Title)
}

func TestCreateTopicRejectsEmptyTitle(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)
	_, err := svc.CreateTopic(ctx, owner, courseID, "  ")
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateTopicRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)
	_, err := svc.CreateTopic(ctx, stranger, courseID, "Atoms")
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestArchiveTopicRemovesItFromList(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)
	require.NoError(t, svc.ArchiveTopic(ctx, owner, topic.ID))

	list, err := svc.ListTopics(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestArchiveTopicRejectsStranger(t *testing.T) {
	svc, ctx, owner, stranger, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	assert.True(t, errors.Is(svc.ArchiveTopic(ctx, stranger, topic.ID), apperr.ErrForbidden))
}

func TestUpdateTopicTitle(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	topic, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	updated, err := svc.UpdateTopic(ctx, owner, topic.ID, "Atomic structure")
	require.NoError(t, err)
	assert.Equal(t, "Atomic structure", updated.Title)
}
