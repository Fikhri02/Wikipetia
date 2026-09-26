package progress_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestFirstEvidenceSetsStrengthDirectly(t *testing.T) {
	// With no history, the first score IS the strength — no smoothing toward zero.
	assert.InDelta(t, 0.8, progress.UpdateStrength(0, 0, 80), 0.0001)
	assert.InDelta(t, 0.0, progress.UpdateStrength(0, 0, 0), 0.0001)
}

func TestLaterEvidenceMovesStrengthPartway(t *testing.T) {
	// alpha = 0.3: prev 0.5, score 100 -> 0.5*0.7 + 1.0*0.3 = 0.65
	assert.InDelta(t, 0.65, progress.UpdateStrength(0.5, 3, 100), 0.0001)
	// A bad result pulls it down but does not erase history.
	assert.InDelta(t, 0.35, progress.UpdateStrength(0.5, 3, 0), 0.0001)
}

func TestStrengthStaysWithinZeroAndOne(t *testing.T) {
	assert.LessOrEqual(t, progress.UpdateStrength(1.0, 10, 100), 1.0)
	assert.GreaterOrEqual(t, progress.UpdateStrength(0.0, 10, 0), 0.0)
}

func TestDecayHalvesStrengthOverTheHalfLife(t *testing.T) {
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	assert.InDelta(t, 0.4, progress.DecayedStrength(0.8, thirtyDaysAgo, now), 0.01,
		"a 30-day half-life halves strength after 30 days")
}

func TestDecayDoesNothingForFreshEvidence(t *testing.T) {
	now := time.Now()
	assert.InDelta(t, 0.8, progress.DecayedStrength(0.8, now, now), 0.0001)
	assert.InDelta(t, 0.8, progress.DecayedStrength(0.8, now.Add(time.Hour), now), 0.0001,
		"a future timestamp must not inflate strength")
}

func TestLevelThresholds(t *testing.T) {
	assert.Equal(t, "new", progress.Level(0, 0))
	assert.Equal(t, "learning", progress.Level(0.2, 1))
	assert.Equal(t, "learning", progress.Level(0.49, 4))
	assert.Equal(t, "proficient", progress.Level(0.5, 4))
	assert.Equal(t, "proficient", progress.Level(0.79, 4))
	assert.Equal(t, "mastered", progress.Level(0.8, 4))
	assert.Equal(t, "mastered", progress.Level(1.0, 9))
}
