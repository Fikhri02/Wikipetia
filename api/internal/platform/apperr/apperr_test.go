package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestSentinelsMatchViaErrorsIs(t *testing.T) {
	assert.True(t, errors.Is(apperr.NotFound("course %s", "abc"), apperr.ErrNotFound))
	assert.True(t, errors.Is(apperr.Forbidden("not yours"), apperr.ErrForbidden))
	assert.True(t, errors.Is(apperr.Conflict("already enrolled"), apperr.ErrConflict))
	assert.True(t, errors.Is(apperr.Invalid("title", "must not be empty"), apperr.ErrValidation))
}

func TestSentinelsDoNotCrossMatch(t *testing.T) {
	assert.False(t, errors.Is(apperr.NotFound("x"), apperr.ErrForbidden))
}

func TestWrappingSurvives(t *testing.T) {
	wrapped := fmt.Errorf("loading course: %w", apperr.NotFound("course abc"))
	assert.True(t, errors.Is(wrapped, apperr.ErrNotFound))
}

func TestInvalidCarriesField(t *testing.T) {
	var ve *apperr.ValidationError
	assert.True(t, errors.As(apperr.Invalid("title", "must not be empty"), &ve))
	assert.Equal(t, "title", ve.Field)
}
