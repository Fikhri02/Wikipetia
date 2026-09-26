// Package enrolment owns the link between a learner and a course. Every
// progress row keys off an enrolment, never a user, which is what lets classes
// arrive in a later slice without touching progress.
package enrolment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

const (
	ModeSelfStudy = "self_study"
	ModeClassroom = "classroom"
)

type Enrolment struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CourseID  uuid.UUID
	Mode      string
	StartedAt time.Time
}

type LibraryEntry struct {
	Enrolment
	CourseTitle string
}

type Service struct {
	runner db.Runner
}

func New(runner db.Runner) *Service {
	return &Service{runner: runner}
}

func (s *Service) Enrol(ctx context.Context, userID, courseID uuid.UUID, mode string) (Enrolment, error) {
	switch mode {
	case ModeSelfStudy:
	case ModeClassroom:
		return Enrolment{}, apperr.Invalid("mode",
			"classroom enrolment requires a class, which arrives in a later slice")
	default:
		return Enrolment{}, apperr.Invalid("mode", "must be %s", ModeSelfStudy)
	}

	var out Enrolment
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		// Slice 1 does not enforce visibility: any existing course may be
		// enrolled in. Access control arrives with slice 5.
		if _, err := q.GetCourseByID(ctx, courseID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.NotFound("course %s", courseID)
			}
			return fmt.Errorf("get course: %w", err)
		}

		row, err := q.CreateEnrolment(ctx, sqlcgen.CreateEnrolmentParams{
			UserID:   userID,
			CourseID: courseID,
			Mode:     mode,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return apperr.Conflict("already enrolled in course %s", courseID)
			}
			return fmt.Errorf("create enrolment: %w", err)
		}
		out = toEnrolment(row)
		return nil
	})
	return out, err
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]LibraryEntry, error) {
	out := []LibraryEntry{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListEnrolmentsByUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("list enrolments: %w", err)
		}
		for _, row := range rows {
			out = append(out, LibraryEntry{
				Enrolment: Enrolment{
					ID:        row.ID,
					UserID:    row.UserID,
					CourseID:  row.CourseID,
					Mode:      row.Mode,
					StartedAt: row.StartedAt,
				},
				CourseTitle: row.CourseTitle,
			})
		}
		return nil
	})
	return out, err
}

// Load resolves an enrolment and checks it belongs to the caller. Progress and
// player reads authorize through this.
func (s *Service) Load(ctx context.Context, q *sqlcgen.Queries, userID, enrolmentID uuid.UUID) (sqlcgen.Enrolment, error) {
	row, err := q.GetEnrolmentByID(ctx, enrolmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Enrolment{}, apperr.NotFound("enrolment %s", enrolmentID)
	}
	if err != nil {
		return sqlcgen.Enrolment{}, fmt.Errorf("get enrolment: %w", err)
	}
	if row.UserID != userID {
		return sqlcgen.Enrolment{}, apperr.Forbidden("enrolment %s is not yours", enrolmentID)
	}
	return row, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func toEnrolment(row sqlcgen.Enrolment) Enrolment {
	return Enrolment{
		ID: row.ID, UserID: row.UserID, CourseID: row.CourseID,
		Mode: row.Mode, StartedAt: row.StartedAt,
	}
}
