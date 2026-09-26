package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/auth"
)

type outlineBlockBody struct {
	blockBody
	TopicIDs      []uuid.UUID `json:"topic_ids"`
	QuestionCount int         `json:"question_count"`
}

type outlineModuleBody struct {
	ID      uuid.UUID          `json:"id"`
	Title   string             `json:"title"`
	SortKey string             `json:"sort_key"`
	Blocks  []outlineBlockBody `json:"blocks"`
}

type outlineOutput struct {
	Body struct {
		Course  courseBody          `json:"course"`
		Topics  []namedBody         `json:"topics"`
		Modules []outlineModuleBody `json:"modules"`
	}
}

func (s *server) registerOutlineRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "getCourseOutline", Method: http.MethodGet, Path: "/courses/{id}/outline",
		Summary: "Get a whole course in one request",
	}, func(ctx context.Context, in *idOnlyInput) (*outlineOutput, error) {
		outline, err := svc.Outline(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}

		out := &outlineOutput{}
		out.Body.Course = courseToBody(outline.Course)

		out.Body.Topics = make([]namedBody, 0, len(outline.Topics))
		for _, topic := range outline.Topics {
			out.Body.Topics = append(out.Body.Topics, namedBody{
				ID: topic.ID, Title: topic.Title, SortKey: topic.SortKey,
			})
		}

		out.Body.Modules = make([]outlineModuleBody, 0, len(outline.Modules))
		for _, mod := range outline.Modules {
			blocks := make([]outlineBlockBody, 0, len(mod.Blocks))
			for _, b := range mod.Blocks {
				blocks = append(blocks, outlineBlockBody{
					blockBody:     blockToBody(b.Block),
					TopicIDs:      b.TopicIDs,
					QuestionCount: b.QuestionCount,
				})
			}
			out.Body.Modules = append(out.Body.Modules, outlineModuleBody{
				ID: mod.Module.ID, Title: mod.Module.Title,
				SortKey: mod.Module.SortKey, Blocks: blocks,
			})
		}
		return out, nil
	})
}
