package main

import (
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// toHTTP is the ONLY place in the codebase that turns a domain error into a
// status code. Handlers return toHTTP(err); domain packages never do.
func toHTTP(err error) error {
	if err == nil {
		return nil
	}

	var ve *apperr.ValidationError
	if errors.As(err, &ve) {
		return &huma.ErrorModel{
			Status: 422,
			Title:  "Unprocessable Entity",
			Detail: "validation failed",
			Errors: []*huma.ErrorDetail{{
				Message:  ve.Msg,
				Location: "body." + ve.Field,
			}},
		}
	}

	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, apperr.ErrForbidden):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, apperr.ErrConflict):
		return huma.Error409Conflict(err.Error())
	}

	ref := uuid.NewString()
	slog.Error("unhandled error", "ref", ref, "err", err)
	return huma.Error500InternalServerError("internal error (ref " + ref + ")")
}
