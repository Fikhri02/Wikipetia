package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutlineReturnsCourseTopicsAndModulesInOneRequest(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	courseID, moduleID := newModule(t, api, "Chemistry")

	topicResp := api.Post("/courses/"+courseID+"/topics", map[string]any{"title": "Atoms"})
	require.Equal(t, http.StatusCreated, topicResp.Code)

	blockResp := api.Post("/modules/"+moduleID+"/blocks", map[string]any{
		"title": "Reading", "type": "reading",
		"content": map[string]any{"body_md": "# Atoms"},
	})
	require.Equal(t, http.StatusCreated, blockResp.Code)

	resp := api.Get("/courses/" + courseID + "/outline")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var parsed struct {
		Course  struct{ Title string } `json:"course"`
		Topics  []struct{ Title string } `json:"topics"`
		Modules []struct {
			Title  string `json:"title"`
			Blocks []struct {
				Title         string   `json:"title"`
				TopicIDs      []string `json:"topic_ids"`
				QuestionCount int      `json:"question_count"`
			} `json:"blocks"`
		} `json:"modules"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &parsed))

	assert.Equal(t, "Chemistry", parsed.Course.Title)
	require.Len(t, parsed.Topics, 1)
	require.Len(t, parsed.Modules, 1)
	require.Len(t, parsed.Modules[0].Blocks, 1)
	assert.Equal(t, "Reading", parsed.Modules[0].Blocks[0].Title)
	assert.Empty(t, parsed.Modules[0].Blocks[0].TopicIDs)
	assert.Equal(t, 0, parsed.Modules[0].Blocks[0].QuestionCount)
}

func TestOutlineOnMissingCourseReturns404(t *testing.T) {
	api, _ := newTestAPI(t, "owner")
	assert.Equal(t, http.StatusNotFound,
		api.Get("/courses/"+uuid.New().String()+"/outline").Code)
}
