package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// StatusNotStarted exists only in the player view. The database has no such
// row; its absence means not started.
const StatusNotStarted = "not_started"

// PlayerQuestion has no Answer field. That is deliberate and structural: the
// type cannot carry a key, so no refactor can accidentally serialise one.
type PlayerQuestion struct {
	ID     uuid.UUID       `json:"id"`
	Kind   string          `json:"kind"`
	Prompt json.RawMessage `json:"prompt"`
}

type PlayerBlock struct {
	ID         uuid.UUID        `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	Content    json.RawMessage  `json:"content"`
	Completion json.RawMessage  `json:"completion"`
	TopicIDs   []uuid.UUID      `json:"topic_ids"`
	Status     string           `json:"status"`
	Score      *int             `json:"score,omitempty"`
	Attempts   int              `json:"attempts"`
	Questions  []PlayerQuestion `json:"questions,omitempty"`
}

type PlayerModule struct {
	ID     uuid.UUID     `json:"id"`
	Title  string        `json:"title"`
	Blocks []PlayerBlock `json:"blocks"`
}

type PlayerTopic struct {
	TopicID  uuid.UUID `json:"topic_id"`
	Title    string    `json:"title"`
	Strength float64   `json:"strength"`
	Level    string    `json:"level"`
}

type PlayerView struct {
	EnrolmentID     uuid.UUID      `json:"enrolment_id"`
	CourseID        uuid.UUID      `json:"course_id"`
	CourseTitle     string         `json:"course_title"`
	PetEnabled      bool           `json:"pet_enabled"`
	Mode            string         `json:"mode"`
	StartedAt       time.Time      `json:"started_at"`
	BlocksTotal     int            `json:"blocks_total"`
	BlocksCompleted int            `json:"blocks_completed"`
	Topics          []PlayerTopic  `json:"topics"`
	Modules         []PlayerModule `json:"modules"`
}

// Player returns everything the learning surface needs in one round trip:
// the course structure, this learner's progress on every block, the questions
// (prompts only), and per-topic mastery with decay applied.
func (s *Service) Player(ctx context.Context, userID, enrolmentID uuid.UUID) (PlayerView, error) {
	var out PlayerView

	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.enrolments.Load(ctx, q, userID, enrolmentID); err != nil {
			return err
		}

		header, err := q.PlayerCourseHeader(ctx, enrolmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("enrolment %s", enrolmentID)
		}
		if err != nil {
			return fmt.Errorf("player header: %w", err)
		}

		out.EnrolmentID = enrolmentID
		out.CourseID = header.CourseID
		out.CourseTitle = header.CourseTitle
		out.PetEnabled = header.PetEnabled
		out.Mode = header.Mode
		out.StartedAt = header.StartedAt

		questionRows, err := q.PlayerQuestions(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player questions: %w", err)
		}
		questionsByBlock := map[uuid.UUID][]PlayerQuestion{}
		for _, row := range questionRows {
			questionsByBlock[row.BlockID] = append(questionsByBlock[row.BlockID], PlayerQuestion{
				ID: row.ID, Kind: row.Kind, Prompt: row.Prompt,
			})
		}

		blockRows, err := q.PlayerBlocks(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player blocks: %w", err)
		}

		// The query orders by module then block sort key, so a single pass
		// builds the modules in order without sorting again.
		var modules []PlayerModule
		indexByModule := map[uuid.UUID]int{}

		for _, row := range blockRows {
			status := StatusNotStarted
			if row.Status != nil {
				status = *row.Status
			}
			var score *int
			if row.Score != nil {
				v := int(*row.Score)
				score = &v
			}
			topicIDs := row.TopicIds
			if topicIDs == nil {
				topicIDs = []uuid.UUID{}
			}

			block := PlayerBlock{
				ID: row.ID, Title: row.Title, Type: row.Type,
				Content: row.Content, Completion: row.Completion,
				TopicIDs: topicIDs, Status: status, Score: score,
				Attempts:  int(row.Attempts),
				Questions: questionsByBlock[row.ID],
			}

			out.BlocksTotal++
			if status == StatusCompleted {
				out.BlocksCompleted++
			}

			idx, seen := indexByModule[row.ModuleID]
			if !seen {
				modules = append(modules, PlayerModule{
					ID: row.ModuleID, Title: row.ModuleTitle, Blocks: []PlayerBlock{},
				})
				idx = len(modules) - 1
				indexByModule[row.ModuleID] = idx
			}
			modules[idx].Blocks = append(modules[idx].Blocks, block)
		}
		if modules == nil {
			modules = []PlayerModule{}
		}
		out.Modules = modules

		topicRows, err := q.PlayerTopics(ctx, enrolmentID)
		if err != nil {
			return fmt.Errorf("player topics: %w", err)
		}
		now := time.Now().UTC()
		out.Topics = make([]PlayerTopic, 0, len(topicRows))
		for _, row := range topicRows {
			strength := row.Strength
			// Decay is computed on read, never stored.
			if last := pgconv.TimePtr(row.LastEvidenceAt); last != nil {
				strength = DecayedStrength(strength, *last, now)
			}
			out.Topics = append(out.Topics, PlayerTopic{
				TopicID:  row.TopicID,
				Title:    row.Title,
				Strength: strength,
				Level:    Level(strength, int(row.EvidenceCount)),
			})
		}
		return nil
	})

	if err != nil {
		return PlayerView{}, err
	}
	return out, nil
}
