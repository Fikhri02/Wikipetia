package main

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type enrolmentBody struct {
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title,omitempty"`
	Mode        string    `json:"mode"`
	StartedAt   time.Time `json:"started_at"`
}

type enrolmentOutput struct {
	Body enrolmentBody
}

type enrolmentListOutput struct {
	Body struct {
		Enrolments []enrolmentBody `json:"enrolments"`
	}
}

type createEnrolmentInput struct {
	Body struct {
		CourseID uuid.UUID `json:"course_id"`
		Mode     string    `json:"mode" enum:"self_study,classroom"`
	}
}

func (s *server) registerEnrolmentRoutes(api huma.API) {
	svc := enrolment.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createEnrolment", Method: http.MethodPost, Path: "/enrolments",
		Summary: "Enrol in a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createEnrolmentInput) (*enrolmentOutput, error) {
		e, err := svc.Enrol(ctx, auth.MustUser(ctx), in.Body.CourseID, in.Body.Mode)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &enrolmentOutput{Body: enrolmentBody{
			ID: e.ID, CourseID: e.CourseID, Mode: e.Mode, StartedAt: e.StartedAt,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listEnrolments", Method: http.MethodGet, Path: "/enrolments",
		Summary: "My library",
	}, func(ctx context.Context, _ *struct{}) (*enrolmentListOutput, error) {
		list, err := svc.ListMine(ctx, auth.MustUser(ctx))
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &enrolmentListOutput{}
		out.Body.Enrolments = make([]enrolmentBody, 0, len(list))
		for _, e := range list {
			out.Body.Enrolments = append(out.Body.Enrolments, enrolmentBody{
				ID: e.ID, CourseID: e.CourseID, CourseTitle: e.CourseTitle,
				Mode: e.Mode, StartedAt: e.StartedAt,
			})
		}
		return out, nil
	})
}
