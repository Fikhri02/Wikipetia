package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

type playerInput struct {
	ID uuid.UUID `path:"id" doc:"Enrolment id"`
}

// The view type is already shaped for the wire, with its own json tags and no
// answer fields, so the handler returns it directly rather than remapping it.
type playerOutput struct {
	Body progress.PlayerView
}

func (s *server) registerPlayerRoutes(api huma.API) {
	svc := progress.New(s.runner, enrolment.New(s.runner))

	huma.Register(api, huma.Operation{
		OperationID: "getEnrolment", Method: http.MethodGet, Path: "/enrolments/{id}",
		Summary: "Everything the learning surface needs, in one request",
	}, func(ctx context.Context, in *playerInput) (*playerOutput, error) {
		view, err := svc.Player(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &playerOutput{Body: view}, nil
	})
}
