package progress

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// Answer is one submitted answer. Exactly one payload field is meaningful,
// chosen by the question's kind.
type Answer struct {
	QuestionID  uuid.UUID
	ChoiceIndex *int
	Boolean     *bool
	Text        *string
}

type QuestionResult struct {
	QuestionID uuid.UUID
	TopicID    *uuid.UUID
	Correct    bool
}

// Grade scores a submission server-side. Answer keys never leave the server,
// so this is the only place correctness is decided.
//
// Unanswered questions count as wrong: a submission is a submission.
func Grade(questions []course.Question, answers []Answer) ([]QuestionResult, int, error) {
	if len(questions) == 0 {
		return nil, 0, apperr.Invalid("block", "this quiz has no questions")
	}

	byID := make(map[uuid.UUID]course.Question, len(questions))
	for _, q := range questions {
		byID[q.ID] = q
	}

	submitted := make(map[uuid.UUID]Answer, len(answers))
	for _, a := range answers {
		if _, ok := byID[a.QuestionID]; !ok {
			return nil, 0, apperr.Invalid("answers",
				"question %s is not part of this quiz", a.QuestionID)
		}
		submitted[a.QuestionID] = a
	}

	results := make([]QuestionResult, 0, len(questions))
	correct := 0
	for _, q := range questions {
		answer, answered := submitted[q.ID]
		isCorrect := false
		if answered {
			var err error
			isCorrect, err = gradeOne(q, answer)
			if err != nil {
				return nil, 0, err
			}
		}
		if isCorrect {
			correct++
		}
		results = append(results, QuestionResult{
			QuestionID: q.ID, TopicID: q.TopicID, Correct: isCorrect,
		})
	}

	score := int(math.Round(float64(correct) / float64(len(questions)) * 100))
	return results, score, nil
}

func gradeOne(q course.Question, a Answer) (bool, error) {
	switch q.Kind {
	case course.QuestionMCQSingle:
		var key course.MCQAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: mcq key for %s: %w", q.ID, err)
		}
		if a.ChoiceIndex == nil {
			return false, nil
		}
		return *a.ChoiceIndex == key.CorrectIndex, nil

	case course.QuestionTrueFalse:
		var key course.BoolAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: true/false key for %s: %w", q.ID, err)
		}
		if a.Boolean == nil {
			return false, nil
		}
		return *a.Boolean == key.Correct, nil

	case course.QuestionTyped:
		var key course.TypedAnswer
		if err := json.Unmarshal(q.Answer, &key); err != nil {
			return false, fmt.Errorf("grading: typed key for %s: %w", q.ID, err)
		}
		if a.Text == nil {
			return false, nil
		}
		given := strings.TrimSpace(*a.Text)
		for _, accepted := range key.Accepted {
			if key.CaseSensitive {
				if given == strings.TrimSpace(accepted) {
					return true, nil
				}
				continue
			}
			if strings.EqualFold(given, strings.TrimSpace(accepted)) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("grading: unknown question kind %q", q.Kind)
	}
}
