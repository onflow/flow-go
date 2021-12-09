package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/arraylinkedlist"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestArrayBackData_SingleBucket(t *testing.T) {
	limit := 10

	bd := NewArrayBackData(uint32(limit), 1, arraylinkedlist.LRUEjection)

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities)

	// sanity checks
	for i := range entities {
		// since we are below limit, elements should be added sequentially at bucket 0.
		// first added element has a key index of 1, since 0 means unused key index in implementation.
		require.Equal(t, bd.buckets[0][i].keyIndex, uint64(i+1))
		// also, since we have not yet over-limited, entities are received valueIndex in the same order they
		// are added.
		require.Equal(t, bd.buckets[0][i].valueIndex, uint32(i))
		_, _, owner := bd.entities.Get(uint32(i))
		require.Equal(t, owner, uint64(i))
	}

	// getting inserted elements
	testRetrievable(t, bd, entities, 0)
}

// TestArrayBackData_WriteHeavy evaluates correctness of backdata under the writing and retrieving
// a heavy load of entities up to its limit.
func TestArrayBackData_WriteHeavy(t *testing.T) {
	limit := 100_000

	bd := NewArrayBackData(uint32(limit), 8, arraylinkedlist.LRUEjection)

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities)

	// retrieves all entities from backdata
	testRetrievable(t, bd, entities, 0)
}

// TestArrayBackData_WriteHeavy evaluates correctness of backdata under the writing and retrieving
// a heavy load of entities up to its limit.
func TestArrayBackData_LRU_Ejection(t *testing.T) {
	// mempool has the limit of 100K, but we put 1M
	// (10 time more than its capacity)
	limit := 100_000
	items := uint(1000_000)

	bd := NewArrayBackData(uint32(limit), 8, arraylinkedlist.LRUEjection)

	entities := unittest.EntityListFixture(items)

	// adds all entities to backdata
	testAddEntities(t, bd, entities)

	// only last 100K items must be retrievable, and
	// the rest must be ejected.
	testRetrievable(t, bd, entities, 900_000)
}

// testAddEntities is a test helper that checks entities are added successfully to the backdata.
// and each entity is retrievable right after it is written to backdata.
func testAddEntities(t *testing.T, bd *ArrayBackData, entities []*unittest.MockEntity) {
	// adding elements
	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, bd.Add(e.ID(), e))

		// total of back data should be incremented by each addition.
		require.Equal(t, bd.Size(), uint(i+1))

		// entity should be placed at index i in back data
		actual, ok := bd.ByID(e.ID())
		require.True(t, ok)
		require.Equal(t, e, actual)
	}
}

// testGettingEntities is a test helper that checks entities are retrievable from backdata.
func testRetrievable(t *testing.T, bd *ArrayBackData, entities []*unittest.MockEntity, from int) {
	for i := range entities {
		expected := entities[i]
		actual, ok := bd.ByID(expected.ID())

		if i < from {
			require.False(t, ok)
			require.Nil(t, actual)
		} else {
			require.True(t, ok)
			require.Equal(t, expected, actual)
		}
	}
}
