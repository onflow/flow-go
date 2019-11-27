package badger

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangelist(t *testing.T) {
	t.Run("should be safe to interact with empty changelist", func(t *testing.T) {
		var clist changelist
		assert.NotPanics(t, func() {
			clist.search(1)
			clist.add(1)
		})
	})

	t.Run("should return notFound", func(t *testing.T) {
		var clist changelist
		// should not find anything when empty
		val := clist.search(1)
		assert.EqualValues(t, notFound, val)

		// should not find anything if all inserted values are greater than input
		clist.add(2)
		clist.add(3)
		val = clist.search(1)
		assert.EqualValues(t, notFound, val)
	})

	t.Run("should be able to find values", func(t *testing.T) {
		// If an exact match exists, should always find that
		t.Run("exact match", func(t *testing.T) {
			var clist changelist

			clist.add(1)
			clist.add(2)

			val := clist.search(1)
			assert.EqualValues(t, 1, val)

			val = clist.search(2)
			assert.EqualValues(t, 2, val)
		})
		// If no exact match exists, should find the highest value that is
		// lower than the input
		t.Run("approx matches", func(t *testing.T) {
			var clist changelist

			clist.add(0)
			clist.add(2)

			val := clist.search(1)
			assert.EqualValues(t, 0, val)

			val = clist.search(3)
			assert.EqualValues(t, 2, val)

			val = clist.search(100000)
			assert.EqualValues(t, 2, val)
		})
	})

	t.Run("should not insert duplicates", func(t *testing.T) {
		var clist changelist

		clist.add(1)
		assert.Equal(t, 1, clist.Len())
		// Length should not change after inserting 1 again
		clist.add(1)
		assert.Equal(t, 1, clist.Len())
	})

	t.Run("should be sorted after every insertion", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))

		var clist changelist
		for i := 0; i < 100; i++ {
			clist.add(r.Uint64())
			assert.True(t, sort.IsSorted(clist))
		}
	})
}

func TestChangelog(t *testing.T) {
	var (
		key1 = "key1"
		key2 = "key2"
	)

	t.Run("should return notFound", func(t *testing.T) {
		clog := newChangelog()

		// should not find anything in empty changelog
		blockNumber := clog.getMostRecentChange(key1, 1)
		assert.EqualValues(t, notFound, blockNumber)
		blockNumber = clog.getMostRecentChange(key2, 2)
		assert.EqualValues(t, notFound, blockNumber)

		// should not find anything for unwritten key
		clog.addChange(key1, 1)
		blockNumber = clog.getMostRecentChange(key2, 1)
		assert.EqualValues(t, notFound, blockNumber)
	})

	t.Run("should find exact block/register matches", func(t *testing.T) {
		clog := newChangelog()

		clog.addChange(key1, 1)
		blockNumber := clog.getMostRecentChange(key1, 1)
		assert.EqualValues(t, 1, blockNumber)
	})

	t.Run("should find approx matches", func(t *testing.T) {
		clog := newChangelog()

		clog.addChange(key1, 1)
		blockNumber := clog.getMostRecentChange(key1, 2)
		assert.EqualValues(t, 1, blockNumber)
	})
}
