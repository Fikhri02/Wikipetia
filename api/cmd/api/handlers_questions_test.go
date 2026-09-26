package main

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newQuizBlock creates a course, a module and a quiz block.
func newQuizBlock(t *testing.T, api humatest.TestAPI) (courseID, blockID string) {
	t.Helper()
	courseID, moduleID := newModule(t, api, "Chemistry")
	blockResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Quiz", "type": "quiz",
	})
	require.Equal(t, http.StatusCreated, blockResp.Code, blockResp.Body.String())
	return courseID, idFromBody(t, blockResp.Body.Bytes())
}

func TestCreateQuestionOnQuizBlockReturns201(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	resp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Which is an element?", "options": []string{"Water", "Iron"}},
		"answer": map[string]any{"correct_index": 1},
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), `"kind":"mcq_single"`)
}

func TestCreateQuestionOnReadingBlockReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, moduleID := newModule(t, api, "Chemistry")

	readingResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Reading", "type": "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, readingResp.Code)
	readingID := idFromBody(t, readingResp.Body.Bytes())

	resp := api.Post("/blocks/"+readingID+"/questions", map[string]any{
		"kind":   "true_false",
		"prompt": map[string]any{"text": "Iron is an element"},
		"answer": map[string]any{"correct": true},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
}

func TestCreateQuestionWithBadAnswerIndexReturns422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	resp := api.Post("/blocks/"+quizID+"/questions", map[string]any{
		"kind":   "mcq_single",
		"prompt": map[string]any{"text": "Pick one", "options": []string{"a", "b"}},
		"answer": map[string]any{"correct_index": 7},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "answer.correct_index")
}

func TestSetBlockTopicsReturnsTheNewSet(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID, quizID := newQuizBlock(t, api)

	topicResp := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, topicResp.Code)
	topicID := idFromBody(t, topicResp.Body.Bytes())

	resp := api.Put("/blocks/"+quizID+"/topics", map[string]any{
		"topic_ids": []string{topicID},
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), topicID)
}

func TestSetBlockTopicsRejectsForeignTopicAt422(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	_, quizID := newQuizBlock(t, api)

	otherCourse := newCourse(t, api, "Physics")
	foreignResp := api.Post("/courses/"+otherCourse+"/topics", map[string]any{"title": "Forces"})
	require.Equal(t, http.StatusCreated, foreignResp.Code)
	foreignTopic := idFromBody(t, foreignResp.Body.Bytes())

	resp := api.Put("/blocks/"+quizID+"/topics", map[string]any{
		"topic_ids": []string{foreignTopic},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), "topic_ids")
}
