package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type blockBody struct {
	ID         uuid.UUID       `json:"id"`
	ModuleID   uuid.UUID       `json:"module_id"`
	Title      string          `json:"title"`
	Type       string          `json:"type"`
	SortKey    string          `json:"sort_key"`
	Content    json.RawMessage `json:"content"`
	Completion json.RawMessage `json:"completion"`
}

func blockToBody(b course.Block) blockBody {
	return blockBody{
		ID: b.ID, ModuleID: b.ModuleID, Title: b.Title, Type: b.Type,
		SortKey: b.SortKey, Content: b.Content, Completion: b.Completion,
	}
}

type blockOutput struct {
	Body blockBody
}

type blockListOutput struct {
	Body struct {
		Blocks []blockBody `json:"blocks"`
	}
}

type createBlockInput struct {
	ID   uuid.UUID `path:"id" doc:"Module id"`
	Body struct {
		Title      string          `json:"title" maxLength:"200"`
		Type       string          `json:"type" enum:"reading,video_youtube,quiz"`
		Content    json.RawMessage `json:"content,omitempty"`
		Completion json.RawMessage `json:"completion,omitempty"`
	}
}

type updateBlockInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		Title      *string         `json:"title,omitempty" maxLength:"200"`
		Content    json.RawMessage `json:"content,omitempty"`
		Completion json.RawMessage `json:"completion,omitempty"`
	}
}

func (s *server) registerBlockRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "createBlock", Method: http.MethodPost, Path: "/modules/{id}/blocks",
		Summary: "Add a block to a module", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createBlockInput) (*blockOutput, error) {
		block, err := svc.CreateBlock(ctx, auth.MustUser(ctx), in.ID, course.CreateBlockInput{
			Title:      in.Body.Title,
			Type:       in.Body.Type,
			Content:    in.Body.Content,
			Completion: in.Body.Completion,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockOutput{Body: blockToBody(block)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listBlocks", Method: http.MethodGet, Path: "/modules/{id}/blocks",
		Summary: "List a module's blocks",
	}, func(ctx context.Context, in *idOnlyInput) (*blockListOutput, error) {
		list, err := svc.ListBlocks(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &blockListOutput{}
		out.Body.Blocks = make([]blockBody, 0, len(list))
		for _, b := range list {
			out.Body.Blocks = append(out.Body.Blocks, blockToBody(b))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateBlock", Method: http.MethodPatch, Path: "/blocks/{id}",
		Summary: "Update a block",
	}, func(ctx context.Context, in *updateBlockInput) (*blockOutput, error) {
		block, err := svc.UpdateBlock(ctx, auth.MustUser(ctx), in.ID, course.UpdateBlockInput{
			Title:      in.Body.Title,
			Content:    in.Body.Content,
			Completion: in.Body.Completion,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &blockOutput{Body: blockToBody(block)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveBlock", Method: http.MethodPost, Path: "/blocks/{id}/archive",
		Summary: "Archive a block", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveBlock(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "moveBlock", Method: http.MethodPost, Path: "/blocks/{id}/move",
		Summary: "Reorder a block", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *moveInput) (*struct{}, error) {
		if err := svc.MoveBlock(ctx, auth.MustUser(ctx), in.ID, in.Body.Before, in.Body.After); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
