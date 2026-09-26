package progress

import (
	"math"
	"time"
)

// The mastery model is deliberately simple and lives in one place, because
// slice 2's spaced-repetition engine replaces it while writing the same column.
// It is an exponentially weighted moving average with time decay. It does NOT
// model forgetting curves; do not treat its output as a retention prediction.
const (
	masteryAlpha        = 0.3  // weight of the newest evidence
	masteryHalfLifeDays = 30.0 // strength halves after this long without evidence

	ProficientThreshold = 0.5
	MasteredThreshold   = 0.8
)

// UpdateStrength folds a new score (0-100) into an existing strength (0-1).
// With no prior evidence the score becomes the strength outright, so a first
// perfect quiz does not read as half-learned.
func UpdateStrength(prev float64, evidenceCount int, score int) float64 {
	s := float64(score) / 100.0
	if evidenceCount <= 0 {
		return clamp01(s)
	}
	return clamp01(prev*(1-masteryAlpha) + s*masteryAlpha)
}

// DecayedStrength applies time decay for display. It is computed on read rather
// than stored, because a stored level goes stale the moment nobody looks at it.
func DecayedStrength(strength float64, lastEvidence, now time.Time) float64 {
	days := now.Sub(lastEvidence).Hours() / 24.0
	if days <= 0 {
		return clamp01(strength)
	}
	return clamp01(strength * math.Pow(0.5, days/masteryHalfLifeDays))
}

// Level names a strength for the UI.
func Level(strength float64, evidenceCount int) string {
	switch {
	case evidenceCount <= 0:
		return "new"
	case strength >= MasteredThreshold:
		return "mastered"
	case strength >= ProficientThreshold:
		return "proficient"
	default:
		return "learning"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
