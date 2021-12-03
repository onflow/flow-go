package backdata

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestArrayBackData_BelowLimit(t *testing.T) {
	size := 10

	bd := NewArrayBackData(uint32(size), 1, LRUEjection)

	entities := unittest.EntityListFixture(uint(size))

	// adding elements
	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, bd.Add(e.ID(), e))

		// total of back data should be incremented by each addition.
		require.Equal(t, bd.Size(), uint(i+1))

		// entity should be placed at index i in back data
		id, entity, _ := bd.entities.get(uint32(i))
		require.Equal(t, e.ID(), id)
		require.Equal(t, e, entity)
	}

	// sanity checks
	for i := range entities {
		// since we are below limit, elements should be added sequentially at bucket 0.
		// first added element has a key index of 1, since 0 means unused key index in implementation.
		require.Equal(t, bd.buckets[0][i].keyIndex, uint64(i+1))
		// also, since we have not yet over-limited, entities are received valueIndex in the same order they
		// are added.
		require.Equal(t, bd.buckets[0][i].valueIndex, uint32(i))
		_, _, owner := bd.entities.get(uint32(i))
		require.Equal(t, owner, uint64(i))
	}

	// getting inserted elements
	for _, expected := range entities {
		actual, ok := bd.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}
}

func TestArrayBackDataStoreAndRetrievalWithoutEjection(t *testing.T) {
	for _, tc := range []struct {
		limit           uint32
		overLimitFactor uint32
		entityCount     uint32
		helpers         []func(*testing.T, *ArrayBackData, []*unittest.MockEntity)
	}{
		{ // two buckets, entities below limit.
			limit:           30,
			overLimitFactor: 2,
			entityCount:     10,
		},
		{ // two buckets, entities equal to limit.
			limit:           30,
			overLimitFactor: 2,
			entityCount:     30,
		},
		{ // multiple buckets, high limit, low entities.
			limit:           10000,
			overLimitFactor: 16,
			entityCount:     1000,
		},
		{ // multiple buckets, entities equal to limit.
			limit:           10000,
			overLimitFactor: 16,
			entityCount:     10000,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-overlimit-%d-entities", tc.limit, tc.overLimitFactor, tc.entityCount), func(t *testing.T) {
			testArrayBackDataStoreAndRetrievalWithoutEjection(t, tc.limit, tc.overLimitFactor, tc.entityCount)
		})
	}
}

func testArrayBackDataStoreAndRetrievalWithoutEjection(t *testing.T, limit uint32, overLimitFactor uint32, entityCount uint32, helpers ...func(*testing.T, *ArrayBackData, []*unittest.MockEntity)) {
	h := []func(*testing.T, *ArrayBackData, []*unittest.MockEntity){
		func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
			testAddingEntities(t, backData, entities)
		},
	}
	h = append(h, helpers...)

	withTestScenario(t, limit, overLimitFactor, entityCount,
		append(h, func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
			testRetrievingSavedEntities(t, backData, entities)
		})...,
	)
}

func testArrayBackDataStoreAndRetrievalWitEjection(t *testing.T, limit uint32, overLimitFactor uint32, entityCount uint32, helpers ...func(*testing.T, *ArrayBackData, []*unittest.MockEntity)) {
	h := []func(*testing.T, *ArrayBackData, []*unittest.MockEntity){
		func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
			testAddingEntities(t, backData, entities)
		},
	}
	h = append(h, helpers...)

	withTestScenario(t, limit, overLimitFactor, entityCount,
		append(h, func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
			testRetrievingSavedEntities(t, backData, entities)
		})...,
	)
}

func withTestScenario(t *testing.T,
	size uint32,
	overLimitFactor uint32,
	entityCount uint32,
	helpers ...func(*testing.T, *ArrayBackData, []*unittest.MockEntity)) {

	bd := NewArrayBackData(size, overLimitFactor, LRUEjection)
	// head on underlying linked list value should be uninitialized
	require.True(t, bd.entities.head.isUndefined())
	require.Equal(t, bd.Size(), uint(0))
	entities := unittest.EntityListFixture(uint(entityCount))

	for _, helper := range helpers {
		helper(t, bd, entities)
	}
}

func testAddingEntities(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
	// adding elements
	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, backData.Add(e.ID(), e))

		// total of back data should be incremented by each addition.
		require.Equal(t, backData.Size(), uint(i+1))

		// entity should be placed at index i in back data
		_, entity, _ := backData.entities.get(uint32(i))
		require.Equal(t, e, entity)

		// linked-list sanity check
		// first insertion forward, head of backData should always point to
		// first entity in the list.
		require.Equal(t, entities[0], backData.entities.getHead().entity)
		require.Equal(t, entities[i], backData.entities.getTail().entity)
		// require.Equal(t, doubleLinkedListPointer(0), backData.entities.getTail().next.sliceIndex())
	}
}

func testRetrievingSavedEntities(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
	for _, expected := range entities {
		actual, ok := backData.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}
}
