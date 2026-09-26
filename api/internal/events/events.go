// Package events is the append-only log the learning side writes facts to and
// the engagement side (slice 4) reads. Writes happen inside the same
// transaction as the state change they describe — the transactional outbox
// pattern — so the two can never disagree.
//
// Delivery to consumers is AT-LEAST-ONCE: a consumer may see the same event
// twice. Every handler that reacts to an event MUST therefore be idempotent.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// Kinds the learning side emits. Engagement adds no kinds: it only consumes.
const (
	KindBlockCompleted = "block.completed"
	KindTopicMastered  = "topic.mastered"
)

var knownKinds = map[string]bool{
	KindBlockCompleted: true,
	KindTopicMastered:  true,
}

type Event struct {
	UserID      uuid.UUID
	EnrolmentID *uuid.UUID
	Kind        string
	Payload     any
}

type Record struct {
	ID          int64
	OccurredAt  time.Time
	UserID      uuid.UUID
	EnrolmentID *uuid.UUID
	Kind        string
	Payload     json.RawMessage
}

// Append writes one event using the caller's transaction. It takes a
// *sqlcgen.Queries rather than a Runner precisely so it cannot open a
// transaction of its own — the event must share the caller's.
func Append(ctx context.Context, q *sqlcgen.Queries, e Event) error {
	if !knownKinds[e.Kind] {
		return fmt.Errorf("events: unknown kind %q", e.Kind)
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("events: marshal payload: %w", err)
	}
	if _, err := q.InsertEvent(ctx, sqlcgen.InsertEventParams{
		UserID:      e.UserID,
		EnrolmentID: pgconv.UUID(e.EnrolmentID), // nullable uuid -> pgtype.UUID
		Kind:        e.Kind,
		Payload:     payload,
	}); err != nil {
		return fmt.Errorf("events: insert: %w", err)
	}
	return nil
}

func ListUnconsumed(ctx context.Context, q *sqlcgen.Queries, limit int32) ([]Record, error) {
	rows, err := q.ListUnconsumedEvents(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("events: list: %w", err)
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, Record{
			ID:          row.ID,
			OccurredAt:  row.OccurredAt,
			UserID:      row.UserID,
			EnrolmentID: pgconv.UUIDPtr(row.EnrolmentID),
			Kind:        row.Kind,
			Payload:     row.Payload,
		})
	}
	return out, nil
}
