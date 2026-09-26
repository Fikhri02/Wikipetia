package course_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
)

func TestPlaceKeyAppendsToEmptyList(t *testing.T) {
	key, err := course.PlaceKey(nil, nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, key)
}

func TestPlaceKeyAppendsToEnd(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, nil, nil)
	require.NoError(t, err)
	assert.Greater(t, key, "B", "appending must sort after the last sibling")
}

func TestPlaceKeyBeforeFirst(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, &a, nil)
	require.NoError(t, err)
	assert.Less(t, key, "A")
}

func TestPlaceKeyAfterLast(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, nil, &b)
	require.NoError(t, err)
	assert.Greater(t, key, "B")
}

func TestPlaceKeyBetweenTwo(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	key, err := course.PlaceKey(sibs, &b, nil)
	require.NoError(t, err)
	assert.Greater(t, key, "A")
	assert.Less(t, key, "B")
}

func TestPlaceKeyRejectsUnknownAnchor(t *testing.T) {
	a := uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}}
	stranger := uuid.New()

	_, err := course.PlaceKey(sibs, &stranger, nil)
	assert.Error(t, err)
}

func TestPlaceKeyRejectsBothAnchors(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	sibs := []course.Ordered{{ID: a, SortKey: "A"}, {ID: b, SortKey: "B"}}

	_, err := course.PlaceKey(sibs, &a, &b)
	assert.Error(t, err, "before and after are mutually exclusive")
}
