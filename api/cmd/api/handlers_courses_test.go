package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCourseReturns201(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "Form 4 Chemistry"})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"visibility":"private"`)
	assert.Contains(t, resp.Body.String(), `"pet_enabled":false`)
}

func TestCreateCourseEmptyTitleReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "   "})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), `"location":"body.title"`)
}

func TestCreateCourseRejectsBadVisibility(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	resp := api.Post("/courses", map[string]any{"title": "C", "visibility": "secret"})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
}

func TestListCoursesReturnsOnlyMine(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	require.Equal(t, http.StatusCreated, api.Post("/courses", map[string]any{"title": "A"}).Code)
	require.Equal(t, http.StatusCreated, api.Post("/courses", map[string]any{"title": "B"}).Code)

	resp := api.Get("/courses")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), `"title":"A"`)
	assert.Contains(t, resp.Body.String(), `"title":"B"`)
}

func TestGetCourseNotFoundReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusNotFound, api.Get("/courses/"+uuid.New().String()).Code)
}

func TestGetCourseMalformedIDReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusUnprocessableEntity, api.Get("/courses/not-a-uuid").Code)
}

func TestPatchCourseLeavesOmittedFields(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	created := api.Post("/courses", map[string]any{"title": "Chemistry", "description": "original"})
	require.Equal(t, http.StatusCreated, created.Code)
	id := idFromBody(t, created.Body.Bytes())

	resp := api.Patch("/courses/"+id, map[string]any{"pet_enabled": true})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"pet_enabled":true`)
	assert.Contains(t, resp.Body.String(), `"description":"original"`)
	assert.Contains(t, resp.Body.String(), `"title":"Chemistry"`)
}

func TestArchiveCourseReturns204ThenGone(t *testing.T) {
	api, _ := newTestAPI(t, "owner")

	created := api.Post("/courses", map[string]any{"title": "Chemistry"})
	require.Equal(t, http.StatusCreated, created.Code)
	id := idFromBody(t, created.Body.Bytes())

	assert.Equal(t, http.StatusNoContent, api.Post("/courses/"+id+"/archive").Code)
	assert.Equal(t, http.StatusNotFound, api.Get("/courses/"+id).Code)
}
