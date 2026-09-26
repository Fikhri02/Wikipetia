package main

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("WikiPetia", "0.1.0"))
	registerHealth(api)

	resp := api.Get("/health")
	require.Equal(t, 200, resp.Code)
	assert.Contains(t, resp.Body.String(), `"status":"ok"`)
}
