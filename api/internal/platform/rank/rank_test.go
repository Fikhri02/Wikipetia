package rank_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/rank"
)

func TestBetweenEmptyBoundsGivesMiddle(t *testing.T) {
	k, err := rank.Between("", "")
	require.NoError(t, err)
	assert.NotEmpty(t, k)
	assert.Equal(t, rank.First(), k)
}

func TestBetweenOrdersCorrectly(t *testing.T) {
	cases := []struct{ prev, next string }{
		{"", ""}, {"V", ""}, {"", "V"}, {"A", "B"}, {"AV", "AW"}, {"0", "1"},
	}
	for _, c := range cases {
		got, err := rank.Between(c.prev, c.next)
		require.NoError(t, err, "Between(%q,%q)", c.prev, c.next)
		if c.prev != "" {
			assert.Greater(t, got, c.prev, "Between(%q,%q)=%q must sort after prev", c.prev, c.next, got)
		}
		if c.next != "" {
			assert.Less(t, got, c.next, "Between(%q,%q)=%q must sort before next", c.prev, c.next, got)
		}
	}
}

func TestBetweenRejectsOutOfOrderBounds(t *testing.T) {
	_, err := rank.Between("B", "A")
	assert.Error(t, err)
}

func TestRepeatedInsertsAtHeadStayOrdered(t *testing.T) {
	keys := []string{}
	head := ""
	for i := 0; i < 50; i++ {
		k, err := rank.Between("", head)
		require.NoError(t, err)
		require.NotEqual(t, head, k)
		keys = append(keys, k)
		head = k
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for i := range keys {
		assert.Equal(t, keys[len(keys)-1-i], sorted[i])
	}
}

func TestRepeatedInsertsBetweenTwoStayOrdered(t *testing.T) {
	lo, hi := "A", "B"
	for i := 0; i < 50; i++ {
		mid, err := rank.Between(lo, hi)
		require.NoError(t, err)
		require.Greater(t, mid, lo)
		require.Less(t, mid, hi)
		hi = mid
	}
}
