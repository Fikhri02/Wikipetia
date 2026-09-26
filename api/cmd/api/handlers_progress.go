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

type submitQuizInput struct {
	EnrolmentID uuid.UUID `path:"enrolment_id"`
	BlockID     uuid.UUID `path:"block_id"`
	Body        struct {
		Answers []struct {
			QuestionID  uuid.UUID `json:"question_id"`
			ChoiceIndex *int      `json:"choice_index,omitempty"`
			Boolean     *bool     `json:"boolean,omitempty"`
			Text        *string   `json:"text,omitempty"`
		} `json:"answers"`
	}
}

// questionResultBody deliberately carries no correct-answer field: the learner
// learns WHETHER they were right, never what the key was.
type questionResultBody struct {
	QuestionID uuid.UUID `json:"question_id"`
	Correct    bool      `json:"correct"`
}

type topicStrengthBody struct {
	TopicID  uuid.UUID `json:"topic_id"`
	Strength float64   `json:"strength"`
	Level    string    `json:"level"`
}

type submitQuizOutput struct {
	Body struct {
		Score    int                  `json:"score"`
		Progress blockProgressBody    `json:"progress"`
		Results  []questionResultBody `json:"results"`
		Mastery  []topicStrengthBody  `json:"mastery"`
	}
}

func (s *server) registerSubmitRoute(api huma.API) {
	svc := progress.New(s.runner, enrolment.New(s.runner))

	huma.Register(api, huma.Operation{
		OperationID: "submitQuiz", Method: http.MethodPost,
		Path:    "/enrolments/{enrolment_id}/blocks/{block_id}/submit",
		Summary: "Submit quiz answers for server-side grading",
	}, func(ctx context.Context, in *submitQuizInput) (*submitQuizOutput, error) {
		answers := make([]progress.Answer, 0, len(in.Body.Answers))
		for _, a := range in.Body.Answers {
			answers = append(answers, progress.Answer{
				QuestionID: a.QuestionID, ChoiceIndex: a.ChoiceIndex,
				Boolean: a.Boolean, Text: a.Text,
			})
		}

		result, err := svc.SubmitQuiz(ctx, auth.MustUser(ctx), in.EnrolmentID, in.BlockID, answers)
		if err != nil {
			return nil, toHTTP(err)
		}

		out := &submitQuizOutput{}
		out.Body.Score = result.Score
		out.Body.Progress = blockProgressBody{
			BlockID: result.Progress.BlockID, Status: result.Progress.Status,
			Score: result.Progress.Score, Attempts: result.Progress.Attempts,
			CompletedAt: result.Progress.CompletedAt,
		}
		out.Body.Results = make([]questionResultBody, 0, len(result.Results))
		for _, r := range result.Results {
			out.Body.Results = append(out.Body.Results, questionResultBody{
				QuestionID: r.QuestionID, Correct: r.Correct,
			})
		}
		out.Body.Mastery = make([]topicStrengthBody, 0, len(result.Mastery))
		for _, m := range result.Mastery {
			out.Body.Mastery = append(out.Body.Mastery, topicStrengthBody{
				TopicID: m.TopicID, Strength: m.Strength, Level: m.Level,
			})
		}
		return out, nil
	})
}
