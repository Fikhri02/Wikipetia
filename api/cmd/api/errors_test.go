package main

import (
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var se huma.StatusError
	require.True(t, errors.As(err, &se), "mapped error must be a huma.StatusError")
	return se.GetStatus()
}

func TestToHTTPMapsSentinels(t *testing.T) {
	assert.Equal(t, 404, statusOf(t, toHTTP(apperr.NotFound("course abc"))))
	assert.Equal(t, 403, statusOf(t, toHTTP(apperr.Forbidden("not yours"))))
	assert.Equal(t, 409, statusOf(t, toHTTP(apperr.Conflict("already enrolled"))))
	assert.Equal(t, 422, statusOf(t, toHTTP(apperr.Invalid("title", "required"))))
}

func TestToHTTPHidesUnexpectedErrors(t *testing.T) {
	err := toHTTP(errors.New("pq: connection reset by peer"))
	assert.Equal(t, 500, statusOf(t, err))
	assert.NotContains(t, err.Error(), "connection reset",
		"internal error text must never reach the client")
}

func TestToHTTPPassesNilThrough(t *testing.T) {
	assert.NoError(t, toHTTP(nil))
}
