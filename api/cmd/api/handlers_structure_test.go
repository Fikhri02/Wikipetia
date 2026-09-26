package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicLifecycleOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	created := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	topicID := idFromBody(t, created.Body.Bytes())

	list := api.Get("/courses/" + courseID + "/topics")
	require.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), `"title":"Atoms"`)

	renamed := api.Patch("/topics/"+topicID, map[string]any{"title": "Atomic structure"})
	require.Equal(t, http.StatusOK, renamed.Code)
	assert.Contains(t, renamed.Body.String(), "Atomic structure")

	assert.Equal(t, http.StatusNoContent, api.Post("/topics/"+topicID+"/archive").Code)

	after := api.Get("/courses/" + courseID + "/topics")
	assert.NotContains(t, after.Body.String(), "Atomic structure")
}

func TestModuleMoveOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID := newCourse(t, api, "Chemistry")

	ids := []string{}
	for _, title := range []string{"One", "Two", "Three"} {
		resp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": title})
		require.Equal(t, http.StatusCreated, resp.Code)
		ids = append(ids, idFromBody(t, resp.Body.Bytes()))
	}

	moved := api.Post("/modules/"+ids[2]+"/move", map[string]any{"before": ids[0]})
	require.Equal(t, http.StatusNoContent, moved.Code, moved.Body.String())

	list := api.Get("/courses/" + courseID + "/modules")
	require.Equal(t, http.StatusOK, list.Code)
	body := list.Body.String()
	assert.Less(t, indexOfTitle(body, "Three"), indexOfTitle(body, "One"))
}

func TestTopicOnMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	resp := api.Post("/courses/"+uuid.New().String()+"/topics", map[string]any{"title": "X"})
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
