package course_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
)

func titles(mods []course.Module) []string {
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		out = append(out, m.Title)
	}
	return out
}

func TestMoveModuleToFront(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"One", "Two", "Three"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	first := created[0].ID
	require.NoError(t, svc.MoveModule(ctx, owner, created[2].ID, &first, nil))

	list, err := svc.ListModules(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Three", "One", "Two"}, titles(list))
}

func TestMoveModuleToEnd(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"One", "Two", "Three"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	require.NoError(t, svc.MoveModule(ctx, owner, created[0].ID, nil, nil))

	list, err := svc.ListModules(ctx, owner, courseID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Two", "Three", "One"}, titles(list))
}

func TestMoveModuleRejectsSelfAnchor(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	m, err := svc.CreateModule(ctx, owner, courseID, "One")
	require.NoError(t, err)

	assert.Error(t, svc.MoveModule(ctx, owner, m.ID, &m.ID, nil))
}

func TestModuleReorderRepeatedlyStaysOrdered(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	var created []course.Module
	for _, title := range []string{"A", "B", "C", "D"} {
		m, err := svc.CreateModule(ctx, owner, courseID, title)
		require.NoError(t, err)
		created = append(created, m)
	}

	head := created[0].ID
	for i := 0; i < 20; i++ {
		require.NoError(t, svc.MoveModule(ctx, owner, created[3].ID, &head, nil))
		list, err := svc.ListModules(ctx, owner, courseID)
		require.NoError(t, err)
		require.Equal(t, "D", list[0].Title, "iteration %d", i)
		head = list[1].ID
	}
}
