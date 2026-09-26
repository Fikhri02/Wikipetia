package main

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

type blockProgressBody struct {
	BlockID     uuid.UUID  `json:"block_id"`
	Status      string     `json:"status"`
	Score       *int       `json:"score,omitempty"`
	Attempts    int        `json:"attempts"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type blockProgressOutput struct {
	Body blockProgressBody
}

type completeBlockInput struct {
	EnrolmentID uuid.UUID `path:"enrolment_id"`
	BlockID     uuid.UUID `path:"block_id"`
	Body        struct {
		MarkedDone   bool `json:"marked_done,omitempty" doc:"The learner pressed done (reading blocks)"`
		WatchedToEnd bool `json:"watched_to_end,omitempty" doc:"The YouTube player reported the end event"`
	}
}

func (s *server) registerProgressRoutes(api huma.API) {
	svc := progress.New(s.runner, enrolment.New(s.runner))

	huma.Register(api, huma.Operation{
		OperationID: "completeBlock", Method: http.MethodPost,
		Path:    "/enrolments/{enrolment_id}/blocks/{block_id}/complete",
		Summary: "Report a completion signal for a reading or video block",
	}, func(ctx context.Context, in *completeBlockInput) (*blockProgressOutput, error) {
		bp, err := svc.MarkComplete(ctx, auth.MustUser(ctx), in.EnrolmentID, in.BlockID, progress.Signal{
			MarkedDone:   in.Body.MarkedDone,
			WatchedToEnd: in.Body.WatchedToEnd,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockProgressOutput{Body: blockProgressBody{
			BlockID: bp.BlockID, Status: bp.Status, Score: bp.Score,
			Attempts: bp.Attempts, CompletedAt: bp.CompletedAt,
		}}, nil
	})
}
