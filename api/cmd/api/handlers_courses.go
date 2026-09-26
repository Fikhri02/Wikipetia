package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type courseBody struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	PetEnabled  bool      `json:"pet_enabled"`
}

func courseToBody(c course.Course) courseBody {
	return courseBody{
		ID:          c.ID,
		Title:       c.Title,
		Description: c.Description,
		Visibility:  c.Visibility,
		PetEnabled:  c.PetEnabled,
	}
}

type createCourseInput struct {
	Body struct {
		Title       string `json:"title" maxLength:"200" doc:"Course title"`
		Description string `json:"description,omitempty" maxLength:"2000"`
		Visibility  string `json:"visibility,omitempty" enum:"public,unlisted,private"`
		PetEnabled  bool   `json:"pet_enabled,omitempty"`
	}
}

type courseOutput struct {
	Body courseBody
}

type courseListOutput struct {
	Body struct {
		Courses []courseBody `json:"courses"`
	}
}

type courseIDInput struct {
	ID uuid.UUID `path:"id"`
}

type updateCourseInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title       *string `json:"title,omitempty" maxLength:"200"`
		Description *string `json:"description,omitempty" maxLength:"2000"`
		Visibility  *string `json:"visibility,omitempty" enum:"public,unlisted,private"`
		PetEnabled  *bool   `json:"pet_enabled,omitempty"`
	}
}

func (s *server) registerCourseRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createCourse", Method: http.MethodPost, Path: "/courses",
		Summary: "Create a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createCourseInput) (*courseOutput, error) {
		c, err := svc.Create(ctx, auth.MustUser(ctx), course.CreateInput{
			Title:       in.Body.Title,
			Description: in.Body.Description,
			Visibility:  in.Body.Visibility,
			PetEnabled:  in.Body.PetEnabled,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listCourses", Method: http.MethodGet, Path: "/courses",
		Summary: "List courses I own",
	}, func(ctx context.Context, _ *struct{}) (*courseListOutput, error) {
		list, err := svc.ListOwned(ctx, auth.MustUser(ctx))
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &courseListOutput{}
		out.Body.Courses = make([]courseBody, 0, len(list))
		for _, c := range list {
			out.Body.Courses = append(out.Body.Courses, courseToBody(c))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getCourse", Method: http.MethodGet, Path: "/courses/{id}",
		Summary: "Get a course",
	}, func(ctx context.Context, in *courseIDInput) (*courseOutput, error) {
		c, err := svc.Get(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateCourse", Method: http.MethodPatch, Path: "/courses/{id}",
		Summary: "Update a course",
	}, func(ctx context.Context, in *updateCourseInput) (*courseOutput, error) {
		c, err := svc.Update(ctx, auth.MustUser(ctx), in.ID, course.UpdateInput{
			Title:       in.Body.Title,
			Description: in.Body.Description,
			Visibility:  in.Body.Visibility,
			PetEnabled:  in.Body.PetEnabled,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &courseOutput{Body: courseToBody(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveCourse", Method: http.MethodPost, Path: "/courses/{id}/archive",
		Summary: "Archive a course", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *courseIDInput) (*struct{}, error) {
		if err := svc.Archive(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
