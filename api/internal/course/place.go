package course

import (
	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/rank"
)

// Ordered is a sibling in a sorted list.
type Ordered struct {
	ID      uuid.UUID
	SortKey string
}

// PlaceKey computes the sort key for an item placed relative to its siblings.
// sibs must already be sorted by SortKey ascending.
//   - before != nil: immediately before that sibling
//   - after  != nil: immediately after that sibling
//   - both nil:      at the end
//
// The client names a neighbour and the server does the arithmetic, so ranking
// logic lives in one place and two tabs cannot mint colliding keys.
func PlaceKey(sibs []Ordered, before, after *uuid.UUID) (string, error) {
	if before != nil && after != nil {
		return "", apperr.Invalid("before", "before and after are mutually exclusive")
	}

	if before == nil && after == nil {
		prev := ""
		if len(sibs) > 0 {
			prev = sibs[len(sibs)-1].SortKey
		}
		if prev == "" {
			return rank.First(), nil
		}
		return rank.Between(prev, "")
	}

	anchor := before
	field := "before"
	if anchor == nil {
		anchor = after
		field = "after"
	}

	idx := -1
	for i, s := range sibs {
		if s.ID == *anchor {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", apperr.Invalid(field, "no sibling with id %s", *anchor)
	}

	if before != nil {
		prev := ""
		if idx > 0 {
			prev = sibs[idx-1].SortKey
		}
		return rank.Between(prev, sibs[idx].SortKey)
	}

	next := ""
	if idx+1 < len(sibs) {
		next = sibs[idx+1].SortKey
	}
	return rank.Between(sibs[idx].SortKey, next)
}
