package pgconv_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
)

func TestUUIDRoundTrip(t *testing.T) {
	id := uuid.New()
	back := pgconv.UUIDPtr(pgconv.UUID(&id))
	require.NotNil(t, back)
	assert.Equal(t, id, *back)
}

func TestUUIDNilRoundTrip(t *testing.T) {
	assert.Nil(t, pgconv.UUIDPtr(pgconv.UUID(nil)))
}

func TestTimeRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	back := pgconv.TimePtr(pgconv.Timestamptz(&now))
	require.NotNil(t, back)
	assert.True(t, now.Equal(*back))
}

func TestTimeNilRoundTrip(t *testing.T) {
	assert.Nil(t, pgconv.TimePtr(pgconv.Timestamptz(nil)))
}
