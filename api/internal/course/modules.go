package course

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Module struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	Title    string
	SortKey  string
}

func (s *Service) CreateModule(ctx context.Context, userID, courseID uuid.UUID, title string) (Module, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Module{}, apperr.Invalid("title", "must not be empty")
	}

	var out Module
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		sibs, err := s.moduleSiblings(ctx, q, courseID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateModule(ctx, sqlcgen.CreateModuleParams{
			CourseID: courseID, Title: title, SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("create module: %w", err)
		}
		out = toModule(row)
		return nil
	})
	return out, err
}

func (s *Service) ListModules(ctx context.Context, userID, courseID uuid.UUID) ([]Module, error) {
	out := []Module{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		rows, err := q.ListModulesByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}
		for _, row := range rows {
			out = append(out, toModule(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateModule(ctx context.Context, userID, moduleID uuid.UUID, title string) (Module, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Module{}, apperr.Invalid("title", "must not be empty")
	}

	var out Module
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadModule(ctx, q, userID, moduleID)
		if err != nil {
			return err
		}
		row, err := q.UpdateModuleTitle(ctx, sqlcgen.UpdateModuleTitleParams{
			ID: existing.ID, Title: title,
		})
		if err != nil {
			return fmt.Errorf("update module: %w", err)
		}
		out = toModule(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveModule(ctx context.Context, userID, moduleID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		n, err := q.ArchiveModule(ctx, moduleID)
		if err != nil {
			return fmt.Errorf("archive module: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("module %s", moduleID)
		}
		return nil
	})
}

// MoveModule repositions a module relative to a sibling. Exactly one of before
// or after must be set; both nil moves it to the end.
func (s *Service) MoveModule(ctx context.Context, userID, moduleID uuid.UUID, before, after *uuid.UUID) error {
	if before != nil && *before == moduleID {
		return apperr.Invalid("before", "cannot move a module relative to itself")
	}
	if after != nil && *after == moduleID {
		return apperr.Invalid("after", "cannot move a module relative to itself")
	}

	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.loadModule(ctx, q, userID, moduleID)
		if err != nil {
			return err
		}
		sibs, err := s.moduleSiblings(ctx, q, row.CourseID)
		if err != nil {
			return err
		}
		// Exclude the module being moved: it must not anchor against itself.
		filtered := make([]Ordered, 0, len(sibs))
		for _, sib := range sibs {
			if sib.ID != moduleID {
				filtered = append(filtered, sib)
			}
		}

		key, err := PlaceKey(filtered, before, after)
		if err != nil {
			return err
		}
		n, err := q.UpdateModuleSortKey(ctx, sqlcgen.UpdateModuleSortKeyParams{
			ID: moduleID, SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("move module: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("module %s", moduleID)
		}
		return nil
	})
}

func (s *Service) loadModule(ctx context.Context, q *sqlcgen.Queries, userID, moduleID uuid.UUID) (sqlcgen.Module, error) {
	row, err := q.GetModuleByID(ctx, moduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Module{}, apperr.NotFound("module %s", moduleID)
	}
	if err != nil {
		return sqlcgen.Module{}, fmt.Errorf("get module: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, row.CourseID); err != nil {
		return sqlcgen.Module{}, err
	}
	return row, nil
}

func (s *Service) moduleSiblings(ctx context.Context, q *sqlcgen.Queries, courseID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListModulesByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toModule(row sqlcgen.Module) Module {
	return Module{ID: row.ID, CourseID: row.CourseID, Title: row.Title, SortKey: row.SortKey}
}
