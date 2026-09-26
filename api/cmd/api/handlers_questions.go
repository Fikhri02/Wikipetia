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

type questionBody struct {
	ID      uuid.UUID       `json:"id"`
	BlockID uuid.UUID       `json:"block_id"`
	TopicID *uuid.UUID      `json:"topic_id,omitempty"`
	Kind    string          `json:"kind"`
	Prompt  json.RawMessage `json:"prompt"`
	Answer  json.RawMessage `json:"answer"`
	SortKey string          `json:"sort_key"`
}

type questionOutput struct {
	Body questionBody
}

type questionListOutput struct {
	Body struct {
		Questions []questionBody `json:"questions"`
	}
}

type createQuestionInput struct {
	ID   uuid.UUID `path:"id" doc:"Block id"`
	Body struct {
		Kind    string          `json:"kind" enum:"mcq_single,true_false,typed"`
		TopicID *uuid.UUID      `json:"topic_id,omitempty"`
		Prompt  json.RawMessage `json:"prompt"`
		Answer  json.RawMessage `json:"answer"`
	}
}

type updateQuestionInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		TopicID *uuid.UUID      `json:"topic_id,omitempty"`
		Prompt  json.RawMessage `json:"prompt,omitempty"`
		Answer  json.RawMessage `json:"answer,omitempty"`
	}
}

type setTopicsInput struct {
	ID   uuid.UUID `path:"id" doc:"Block id"`
	Body struct {
		TopicIDs []uuid.UUID `json:"topic_ids"`
	}
}

type topicIDsOutput struct {
	Body struct {
		TopicIDs []uuid.UUID `json:"topic_ids"`
	}
}

func questionToBody(q course.Question) questionBody {
	return questionBody{
		ID: q.ID, BlockID: q.BlockID, TopicID: q.TopicID, Kind: q.Kind,
		Prompt: q.Prompt, Answer: q.Answer, SortKey: q.SortKey,
	}
}

func (s *server) registerQuestionRoutes(api huma.API) {
	svc := course.New(s.runner)

	huma.Register(api, huma.Operation{
		OperationID: "setBlockTopics", Method: http.MethodPut, Path: "/blocks/{id}/topics",
		Summary: "Replace a block's topic tags",
	}, func(ctx context.Context, in *setTopicsInput) (*topicIDsOutput, error) {
		if err := svc.SetBlockTopics(ctx, auth.MustUser(ctx), in.ID, in.Body.TopicIDs); err != nil {
			return nil, toHTTP(err)
		}
		ids, err := svc.GetBlockTopics(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &topicIDsOutput{}
		out.Body.TopicIDs = ids
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createQuestion", Method: http.MethodPost, Path: "/blocks/{id}/questions",
		Summary: "Add a question to a quiz block", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createQuestionInput) (*questionOutput, error) {
		q, err := svc.CreateQuestion(ctx, auth.MustUser(ctx), in.ID, course.QuestionInput{
			Kind: in.Body.Kind, TopicID: in.Body.TopicID,
			Prompt: in.Body.Prompt, Answer: in.Body.Answer,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &questionOutput{Body: questionToBody(q)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listQuestions", Method: http.MethodGet, Path: "/blocks/{id}/questions",
		Summary: "List a quiz block's questions, with answers (author only)",
	}, func(ctx context.Context, in *idOnlyInput) (*questionListOutput, error) {
		list, err := svc.ListQuestionsForAuthor(ctx, auth.MustUser(ctx), in.ID)
		if err != nil {
			return nil, toHTTP(err)
		}
		out := &questionListOutput{}
		out.Body.Questions = make([]questionBody, 0, len(list))
		for _, q := range list {
			out.Body.Questions = append(out.Body.Questions, questionToBody(q))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateQuestion", Method: http.MethodPatch, Path: "/questions/{id}",
		Summary: "Update a question",
	}, func(ctx context.Context, in *updateQuestionInput) (*questionOutput, error) {
		q, err := svc.UpdateQuestion(ctx, auth.MustUser(ctx), in.ID, course.QuestionInput{
			TopicID: in.Body.TopicID, Prompt: in.Body.Prompt, Answer: in.Body.Answer,
		})
		if err != nil {
			return nil, toHTTP(err)
		}
		return &questionOutput{Body: questionToBody(q)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveQuestion", Method: http.MethodPost, Path: "/questions/{id}/archive",
		Summary: "Archive a question", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idOnlyInput) (*struct{}, error) {
		if err := svc.ArchiveQuestion(ctx, auth.MustUser(ctx), in.ID); err != nil {
			return nil, toHTTP(err)
		}
		return nil, nil
	})
}
