package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrolReturns201AndAppearsInLibrary(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	resp := api.Post("/enrolments", map[string]any{
		"course_id": courseID, "mode": "self_study",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	library := api.Get("/enrolments")
	require.Equal(t, http.StatusOK, library.Code)
	assert.Contains(t, library.Body.String(), `"course_title":"Chemistry"`)
}

func TestEnrolTwiceReturns409(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	first := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "self_study"})
	require.Equal(t, http.StatusCreated, first.Code)

	second := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "self_study"})
	assert.Equal(t, http.StatusConflict, second.Code, second.Body.String())
}

func TestEnrolInClassroomModeReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "learner")
	courseID := newCourse(t, api, "Chemistry")

	resp := api.Post("/enrolments", map[string]any{"course_id": courseID, "mode": "classroom"})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
}

func TestEnrolInMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "learner")

	resp := api.Post("/enrolments", map[string]any{
		"course_id": uuid.New().String(), "mode": "self_study",
	})
	assert.Equal(t, http.StatusNotFound, resp.Code)
}
