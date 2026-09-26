package main

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newModule creates a course and a module, returning both ids.
func newModule(t *testing.T, api humatest.TestAPI, courseTitle string) (courseID, moduleID string) {
	t.Helper()
	courseID = newCourse(t, api, courseTitle)
	resp := api.Post("/courses/"+courseID+"/modules", map[string]any{"title": "Module 1"})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	return courseID, idFromBody(t, resp.Body.Bytes())
}

func TestCreateBlockOverHTTP(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, moduleID := newModule(t, api, "Chemistry")

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title":   "What is an atom",
		"type":    "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"kind":"mark_done"`)
}

func TestCreateBlockRejectsUnknownTypeAt422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, moduleID := newModule(t, api, "Chemistry")

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "X", "type": "hologram",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code,
		"huma's enum tag rejects it before the domain does")
}

func TestCreateVideoBlockWithoutVideoIDReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, moduleID := newModule(t, api, "Chemistry")

	resp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Video", "type": "video_youtube", "content": map[string]any{},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "content.video_id")
}
