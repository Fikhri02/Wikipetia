package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Store struct{}

// EnsureBySubject maps an identity-provider subject to our own user row,
// creating it on first sight. Keeping our own users table is what makes the
// auth provider swappable.
func (Store) EnsureBySubject(ctx context.Context, q *sqlcgen.Queries, subject, name string) (uuid.UUID, error) {
	existing, err := q.GetUserBySubject(ctx, &subject)
	switch {
	case err == nil:
		return existing.ID, nil
	case errors.Is(err, pgx.ErrNoRows):
		created, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
			AuthSubject: &subject,
			Name:        name,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("create user: %w", err)
		}
		return created.ID, nil
	default:
		return uuid.Nil, fmt.Errorf("get user: %w", err)
	}
}
