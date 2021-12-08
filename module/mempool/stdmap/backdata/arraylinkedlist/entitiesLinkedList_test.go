package arraylinkedlist

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestStoreAndRetrievalWithoutEjection(t *testing.T) {
	for _, tc := range []struct {
		limit           uint32
		overLimitFactor uint32
		entityCount     uint32
		helpers         []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity)
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
			limit:           2000,
			overLimitFactor: 16,
			entityCount:     1000,
		},
		{ // multiple buckets, entities equal to limit.
			limit:           1000,
			overLimitFactor: 16,
			entityCount:     1000,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-overlimit-%d-entities", tc.limit, tc.overLimitFactor, tc.entityCount), func(t *testing.T) {
			testArrayBackDataStoreAndRetrievalWithoutEjection(t, tc.limit, tc.entityCount)
		})
	}
}

func testArrayBackDataStoreAndRetrievalWithoutEjection(t *testing.T, limit uint32, entityCount uint32, helpers ...func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity)) {
	h := []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
		func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
			testInitialization(t, list, entities)
		},
		func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
			testAddingEntities(t, list, entities)
		},
	}
	h = append(h, helpers...)

	withTestScenario(t, limit, entityCount,
		append(h, func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
			testRetrievingLastXSavedEntities(t, list, entities, 0)
		})...,
	)
}

func TestArrayBackDataStoreAndRetrievalWithEjection(t *testing.T) {
	for _, tc := range []struct {
		limit           uint32
		overLimitFactor uint32
		entityCount     uint32
		helpers         []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity)
	}{
		{
			limit:           30,
			overLimitFactor: 2,
			entityCount:     31,
		},
		{
			limit:           30,
			overLimitFactor: 2,
			entityCount:     100,
		},
		{
			limit:           1000,
			overLimitFactor: 8,
			entityCount:     2000,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-overlimit-%d-entities", tc.limit, tc.overLimitFactor, tc.entityCount), func(t *testing.T) {
			testArrayBackDataStoreAndRetrievalWitEjection(t, tc.limit, tc.entityCount)
		})
	}
}

func testArrayBackDataStoreAndRetrievalWitEjection(t *testing.T, limit uint32, entityCount uint32, helpers ...func(*testing.T, *EntityDoubleLinkedList,
	[]*unittest.MockEntity)) {
	h := []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
		func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
			testAddingEntities(t, backData, entities)
		},
	}
	h = append(h, helpers...)

	withTestScenario(t, limit, entityCount,
		append(h, func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
			testRetrievingLastXSavedEntities(t, list, entities, entityCount-limit)
		})...,
	)
}

// testInitialization evaluates the state of an initialized cachedEntity list before adding any element
// to it.
func testInitialization(t *testing.T, list *EntityDoubleLinkedList, _ []*unittest.MockEntity) {
	// head and tail of "used" linked-list must be undefined at initialization time.
	require.True(t, list.used.head.isUndefined())
	require.True(t, list.used.tail.isUndefined())

	for i := 0; i < len(list.values); i++ {
		if i == 0 {
			// head of embedded "free" linked-list should point to index 0 of entities slice.
			require.Equal(t, uint32(i), list.free.head.sliceIndex())
			// previous element of tail must be undefined.
			require.True(t, list.values[i].prev.isUndefined())
		}

		if i != 0 {
			// except head, any element should point back to its previous index in slice.
			require.Equal(t, uint32(i-1), list.values[i].prev.sliceIndex())
		}

		if i != len(list.values)-1 {
			// except tail, any element should point forward to its next index in slice.
			require.Equal(t, uint32(i+1), list.values[i].next.sliceIndex())
		}

		if i == len(list.values)-1 {
			// tail of embedded "free" linked-list should point to the last index in entities slice.
			require.Equal(t, uint32(i), list.free.tail.sliceIndex())
			// next element of tail must be undefined.
			require.True(t, list.values[i].next.isUndefined())
		}
	}
}

func testAddingEntities(t *testing.T, list *EntityDoubleLinkedList, entitiesToBeAdded []*unittest.MockEntity) {
	// adding elements
	for i, e := range entitiesToBeAdded {
		// adding each element must be successful.
		list.Add(e.ID(), e, uint64(i))

		// in case of no over limit, total of back data should be incremented by each addition.
		if i < len(list.values) {
			require.Equal(t, list.Size(), uint32(i+1))
		}

		// entity should be placed at index i in back data
		_, entity, _ := list.Get(uint32(i % len(list.values)))
		require.Equal(t, e, entity)

		// linked-list sanity check
		// first insertion forward, head of backData should always point to
		// first entity in the list.
		usedHead, freeHead := list.getHeads()
		usedTail, freeTail := list.getTails()

		//
		expectedUsedHead := 0
		if i >= len(list.values) {
			expectedUsedHead = (i + 1) % len(list.values)
		}
		require.Equal(t, list.values[expectedUsedHead].entity, usedHead.entity)
		require.True(t, usedHead.prev.isUndefined())

		//
		require.Equal(t, entitiesToBeAdded[i], usedTail.entity)
		require.True(t, usedTail.next.isUndefined())

		// free head
		// as long as we are below limit, after adding i element, free head
		// should move to i+1 element.
		if i < len(list.values)-1 {
			require.Equal(t, uint32(i+1), list.free.head.sliceIndex())
			require.True(t, freeHead.prev.isUndefined())
		} else {
			require.Nil(t, freeHead)
		}

		// free tail
		if i < len(list.values)-1 {
			require.Equal(t, uint32(len(list.values)-1), list.free.tail.sliceIndex())
			require.True(t, freeTail.next.isUndefined())
		} else {
			require.Nil(t, freeTail)
		}

		// used entitiesToBeAdded list
		// if we are still below limit, head to tail of used list
		// must be reachable within i + 1 steps.
		usedTraverseStep := uint32(i + 1)
		if i >= len(list.values) {
			// if we are above the limit, head to tail of used list
			// must be reachable within as many steps as the actual capacity of
			// list.
			usedTraverseStep = uint32(len(list.values))
		}
		tailAccessibleFromHead(t,
			list.used.head.sliceIndex(),
			list.used.tail.sliceIndex(),
			list,
			usedTraverseStep)
		headAccessibleFromTail(t,
			list.used.head.sliceIndex(),
			list.used.tail.sliceIndex(),
			list,
			usedTraverseStep)

		// free entitiesToBeAdded list
		// if we are still below limit, head to tail of used list
		// must be reachable within limit - i - 1 steps. "limit - i" part is since
		// when we have i elements in list, we have "limit - i" free slots, and -1 is
		// since we start from index 0 not 1.
		freeTraverseStep := uint32(len(list.values) - i - 1)
		if i >= len(list.values) {
			// if we are above the limit, within 0 steps.
			// reason is list is full and adding new elements is done
			// by ejecting existing ones, remaining no free slot.
			freeTraverseStep = uint32(0)
		}
		tailAccessibleFromHead(t,
			list.free.head.sliceIndex(),
			list.free.tail.sliceIndex(),
			list,
			freeTraverseStep)
		headAccessibleFromTail(t,
			list.free.head.sliceIndex(),
			list.free.tail.sliceIndex(),
			list,
			freeTraverseStep)
	}
}

func testRetrievingLastXSavedEntities(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity, from uint32) {
	for i := from; i < uint32(len(entities)); i++ {
		actualID, actual, _ := list.Get(i % uint32(len(list.values)))
		require.Equal(t, entities[i].ID(), actualID)
		require.Equal(t, entities[i], actual)
	}
}

func withTestScenario(t *testing.T,
	limit uint32,
	entityCount uint32,
	helpers ...func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity)) {

	list := NewEntityList(limit, LRUEjection)

	// head on underlying linked list value should be uninitialized
	require.True(t, list.used.head.isUndefined())
	require.Equal(t, list.Size(), uint32(0))

	entities := unittest.EntityListFixture(uint(entityCount))

	for _, helper := range helpers {
		helper(t, list, entities)
	}
}

func tailAccessibleFromHead(t *testing.T, headSliceIndex uint32, tailSliceIndex uint32, list *EntityDoubleLinkedList, total uint32) {
	seen := make(map[uint32]struct{})

	index := headSliceIndex
	for i := uint32(0); i < total; i++ {
		if i == total-1 {
			require.Equal(t, tailSliceIndex, index, "tail not reachable after total steps")
			return
		}

		require.NotEqual(t, tailSliceIndex, index, "tail visited in less expected steps (potential inconsistency)", i, total)
		_, ok := seen[index]
		require.False(t, ok, "duplicate identifiers found")

		require.False(t, list.values[index].next.isUndefined(), "tail not found, and reached end of list")
		index = list.values[index].next.sliceIndex()
	}
}

func headAccessibleFromTail(t *testing.T, headSliceIndex uint32, tailSliceIndex uint32, list *EntityDoubleLinkedList, total uint32) {
	seen := make(map[uint32]struct{})

	index := tailSliceIndex
	for i := uint32(0); i < total; i++ {
		if i == total-1 {
			require.Equal(t, headSliceIndex, index, "head not reachable after total steps")
			return
		}

		require.NotEqual(t, headSliceIndex, index, "head visited in less expected steps (potential inconsistency)", i, total)
		_, ok := seen[index]
		require.False(t, ok, "duplicate identifiers found")

		index = list.values[index].prev.sliceIndex()
	}
}
