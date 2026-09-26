package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type namedBody struct {
	ID      uuid.UUID `json:"id"`
	Title   string    `json:"title"`
	SortKey string    `json:"sort_key"`
}

type namedListOutput struct {
	Body struct {
		Items []namedBody `json:"items"`
	}
}

type namedOutput struct {
	Body namedBody
}

type createNamedInput struct {
	ID   uuid.UUID `path:"id" doc:"Course id"`
	Body struct {
		Title string `json:"title" maxLength:"200"`
	}
}

type renameInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title string `json:"title" maxLength:"200"`
	}
}

type idOnlyInput struct {
	ID uuid.UUID `path:"id"`
}

type moveInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Before *uuid.UUID `json:"before,omitempty" doc:"Place immediately before this sibling"`
		After  *uuid.UUID `json:"after,omitempty" doc:"Place immediately after this sibling"`
	}
}

func (s *server) registerStructureRoutes(api huma.API) {
	svc := course.New(s.runner)

	// ---- Topics ----

	huma.Register(api, huma.Operation{
		OperationID: "createTopic", Method: http.MethodPost, Path: "/courses/{id}/topics",
		Summary: "Add a topic to a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createNamedInput) (*namedOutput, error) {
		topic, err := svc.CreateTopic(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTopics", Method: http.MethodGet, Path: "/courses/{id}/topics",
		Summary: "List a course's topics",
	}, func(ctx context.Context, in *idOnlyInput) (*namedListOutput, error) {
		list, err := svc.ListTopics(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &namedListOutput{}
		out.Body.Items = make([]namedBody, 0, len(list))
		for _, topic := range list {
			out.Body.Items = append(out.Body.Items, namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTopic", Method: http.MethodPatch, Path: "/topics/{id}",
		Summary: "Rename a topic",
	}, func(ctx context.Context, in *renameInput) (*namedOutput, error) {
		topic, err := svc.UpdateTopic(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveTopic", Method: http.MethodPost, Path: "/topics/{id}/archive",
		Summary: "Archive a topic", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveTopic(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	// ---- Modules ----

	huma.Register(api, huma.Operation{
		OperationID: "createModule", Method: http.MethodPost, Path: "/courses/{id}/modules",
		Summary: "Add a module to a course", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createNamedInput) (*namedOutput, error) {
		mod, err := svc.CreateModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listModules", Method: http.MethodGet, Path: "/courses/{id}/modules",
		Summary: "List a course's modules",
	}, func(ctx context.Context, in *idOnlyInput) (*namedListOutput, error) {
		list, err := svc.ListModules(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &namedListOutput{}
		out.Body.Items = make([]namedBody, 0, len(list))
		for _, mod := range list {
			out.Body.Items = append(out.Body.Items, namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey})
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateModule", Method: http.MethodPatch, Path: "/modules/{id}",
		Summary: "Rename a module",
	}, func(ctx context.Context, in *renameInput) (*namedOutput, error) {
		mod, err := svc.UpdateModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Title)
		if err != nil {
			return nil, toHTTP(err)
		}
		return &namedOutput{Body: namedBody{ID: mod.ID, Title: mod.Title, SortKey: mod.SortKey}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveModule", Method: http.MethodPost, Path: "/modules/{id}/archive",
		Summary: "Archive a module", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveModule(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moveModule", Method: http.MethodPost, Path: "/modules/{id}/move",
		Summary: "Reorder a module", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveInput) (*struct{}, error) {
		if err := svc.MoveModule(ctx, auth.MustUser(ctx), in.ID, in.Body.Before, in.Body.After); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
