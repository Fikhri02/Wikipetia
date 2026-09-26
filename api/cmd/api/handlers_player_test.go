package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerEndpointOmitsAnswerKeys(t *testing.T) {
	api, _ := newTestAPI(t, "learner")

	courseID, moduleID := newModule(t, api, "Chemistry")

	quizResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Quiz", "type": "quiz",
	})
	require.Equal(t, http.StatusCreated, quizResp.Code)
	quizID := idFromBody(t, quizResp.Body.Bytes())

	qResp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Which is an element?", "options": []string{"Water", "Iron"}},
		"answer": map[string]any{"correct_index": 1},
	})
	require.Equal(t, http.StatusCreated, qResp.Code, qResp.Body.String())

	enrolResp := api.Post("/enrolments", map[string]any{
		"course_id": courseID, "mode": "self_study",
	})
	require.Equal(t, http.StatusCreated, enrolResp.Code, enrolResp.Body.String())
	enrolmentID := idFromBody(t, enrolResp.Body.Bytes())

	playerResp := api.Get("/enrolments/" + enrolmentID)
	require.Equal(t, http.StatusOK, playerResp.Code, playerResp.Body.String())

	body := playerResp.Body.String()
	assert.Contains(t, body, "Which is an element?", "prompts must be present")
	assert.Contains(t, body, "Iron", "options must be present")
	assert.NotContains(t, body, "correct_index",
		"the answer key must never reach the learner over HTTP")
}
