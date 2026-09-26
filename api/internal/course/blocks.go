package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Block struct {
	ID         uuid.UUID
	ModuleID   uuid.UUID
	Title      string
	Type       string
	SortKey    string
	Content    json.RawMessage
	Completion json.RawMessage
}

type CreateBlockInput struct {
	Title      string
	Type       string
	Content    json.RawMessage
	Completion json.RawMessage
}

type UpdateBlockInput struct {
	Title      *string
	Content    json.RawMessage
	Completion json.RawMessage
}

func (s *Service) CreateBlock(ctx context.Context, userID, moduleID uuid.UUID, in CreateBlockInput) (Block, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Block{}, apperr.Invalid("title", "must not be empty")
	}
	content, completion, err := ValidateBlock(in.Type, in.Content, in.Completion)
	if err != nil {
		return Block{}, err
	}

	var out Block
	err = s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		sibs, err := s.blockSiblings(ctx, q, moduleID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateBlock(ctx, sqlcgen.CreateBlockParams{
			ModuleID:   moduleID,
			Title:      title,
			Type:       in.Type,
			SortKey:    key,
			Content:    content,
			Completion: completion,
		})
		if err != nil {
			return fmt.Errorf("create block: %w", err)
		}
		out = toBlock(row)
		return nil
	})
	return out, err
}

func (s *Service) ListBlocks(ctx context.Context, userID, moduleID uuid.UUID) ([]Block, error) {
	out := []Block{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadModule(ctx, q, userID, moduleID); err != nil {
			return err
		}
		rows, err := q.ListBlocksByModule(ctx, moduleID)
		if err != nil {
			return fmt.Errorf("list blocks: %w", err)
		}
		for _, row := range rows {
			out = append(out, toBlock(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateBlock(ctx context.Context, userID, blockID uuid.UUID, in UpdateBlockInput) (Block, error) {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return Block{}, apperr.Invalid("title", "must not be empty")
	}

	var out Block
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}

		// Re-validate against the block's own type. Whichever of content and
		// completion is absent falls back to what is stored, so a partial
		// update can never leave an invalid pair behind.
		content := in.Content
		if len(content) == 0 {
			content = existing.Content
		}
		completion := in.Completion
		if len(completion) == 0 {
			completion = existing.Completion
		}
		validContent, validCompletion, err := ValidateBlock(existing.Type, content, completion)
		if err != nil {
			return err
		}

		row, err := q.UpdateBlock(ctx, sqlcgen.UpdateBlockParams{
			ID:         blockID,
			Title:      trimPtr(in.Title),
			Content:    validContent,
			Completion: validCompletion,
		})
		if err != nil {
			return fmt.Errorf("update block: %w", err)
		}
		out = toBlock(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveBlock(ctx context.Context, userID, blockID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		n, err := q.ArchiveBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("archive block: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("block %s", blockID)
		}
		return nil
	})
}

func (s *Service) MoveBlock(ctx context.Context, userID, blockID uuid.UUID, before, after *uuid.UUID) error {
	if before != nil && *before == blockID {
		return apperr.Invalid("before", "cannot move a block relative to itself")
	}
	if after != nil && *after == blockID {
		return apperr.Invalid("after", "cannot move a block relative to itself")
	}

	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}
		sibs, err := s.blockSiblings(ctx, q, row.ModuleID)
		if err != nil {
			return err
		}
		filtered := make([]Ordered, 0, len(sibs))
		for _, sib := range sibs {
			if sib.ID != blockID {
				filtered = append(filtered, sib)
			}
		}
		key, err := PlaceKey(filtered, before, after)
		if err != nil {
			return err
		}
		n, err := q.UpdateBlockSortKey(ctx, sqlcgen.UpdateBlockSortKeyParams{ID: blockID, SortKey: key})
		if err != nil {
			return fmt.Errorf("move block: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("block %s", blockID)
		}
		return nil
	})
}

// loadBlock resolves a block and checks the caller owns the course it sits in.
func (s *Service) loadBlock(ctx context.Context, q *sqlcgen.Queries, userID, blockID uuid.UUID) (sqlcgen.Block, error) {
	row, err := q.GetBlockByID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Block{}, apperr.NotFound("block %s", blockID)
	}
	if err != nil {
		return sqlcgen.Block{}, fmt.Errorf("get block: %w", err)
	}
	courseID, err := q.GetBlockCourseID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Block{}, apperr.NotFound("block %s", blockID)
	}
	if err != nil {
		return sqlcgen.Block{}, fmt.Errorf("get block course: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
		return sqlcgen.Block{}, err
	}
	return row, nil
}

func (s *Service) blockSiblings(ctx context.Context, q *sqlcgen.Queries, moduleID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListBlocksByModule(ctx, moduleID)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toBlock(row sqlcgen.Block) Block {
	return Block{
		ID:         row.ID,
		ModuleID:   row.ModuleID,
		Title:      row.Title,
		Type:       row.Type,
		SortKey:    row.SortKey,
		Content:    row.Content,
		Completion: row.Completion,
	}
}
