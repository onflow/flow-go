package arraylinkedlist

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestStoreAndRetrieval_Without_Ejection checks health of entity linked list for storing and retrieval scenarios that
// do not involve ejection.
// The test involves cases for testing the list below its limit, and also up to its limit. However, it never gets beyond
// the limit of list, so no ejection will kick-in.
func TestStoreAndRetrieval_Without_Ejection(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of entity list
		entityCount uint32 // total entities to be stored
	}{
		{
			limit:       30,
			entityCount: 10,
		},
		{
			limit:       30,
			entityCount: 30,
		},
		{
			limit:       2000,
			entityCount: 1000,
		},
		{
			limit:       1000,
			entityCount: 1000,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-entities", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testInitialization(t, list, entities)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, list, entities, LRUEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testRetrievingEntitiesFrom(t, list, entities, 0)
				},
			}...,
			)
		})
	}
}

// TestStoreAndRetrieval_With_LRU_Ejection checks health of entity linked list for storing and retrieval scenarios that
// involves the LRU ejection.
// The test involves cases for testing the list beyond its limit, so the LRU ejection will kick-in.
func TestStoreAndRetrieval_With_LRU_Ejection(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of entity list
		entityCount uint32 // total entities to be stored
	}{
		{
			limit:       30,
			entityCount: 31,
		},
		{
			limit:       30,
			entityCount: 100,
		},
		{
			limit:       1000,
			entityCount: 2000,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-entities", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					// with a limit of tc.limit, storing a total of tc.entityCount (> tc.limit) entities, results
					// in ejection of the first tc.entityCount - tc.limit entities.
					// Hence, we check retrieval of the last tc.limit entities, which start from index
					// tc.entityCount - tc.limit entities.
					testRetrievingEntitiesFrom(t, list, entities, EIndex(tc.entityCount-tc.limit))
				},
			}...,
			)
		})
	}
}

// TestStoreAndRetrieval_With_Random_Ejection checks health of entity linked list for storing and retrieval scenarios that
// involves the LRU ejection.
func TestStoreAndRetrieval_With_Random_Ejection(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of entity list
		entityCount uint32 // total entities to be stored
	}{
		{
			limit:       30,
			entityCount: 31,
		},
		{
			limit:       30,
			entityCount: 100,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-entities", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, RandomEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, RandomEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					// with a limit of tc.limit, storing a total of tc.entityCount (> tc.limit) entities, results
					// in ejection of "tc.entityCount - tc.limit" entities at random.
					// Hence, we check retrieval any successful total of "tc.limit" entities.
					testRetrievingCount(t, list, entities, int(tc.limit))
				},
			}...,
			)
		})
	}
}

// TestInvalidateEntity checks the health of entity linked list for invalidating entities under random, LRU, and LIFO scenarios.
// Invalidating an entity removes it from the linked list and moves its node from used list to free list.
func TestInvalidateEntity(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of entity list
		entityCount uint32 // total entities to be stored
	}{
		{
			limit:       30,
			entityCount: 0,
		},
		{
			limit:       30,
			entityCount: 1,
		},
		{
			limit:       30,
			entityCount: 10,
		},
		{
			limit:       30,
			entityCount: 30,
		},
		{
			limit:       100,
			entityCount: 10,
		},
		{
			limit:       100,
			entityCount: 100,
		},
	} {
		// head invalidation test (LRU)
		t.Run(fmt.Sprintf("head-invalidation-%d-limit-%d-entities", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testInvalidatingHead(t, list, entities)
				},
			}...)
		})

		// tail invalidation test (LIFO)
		t.Run(fmt.Sprintf("tail-invalidation-%d-limit-%d-entities-", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testInvalidatingTail(t, list, entities)
				},
			}...)
		})

		// random invalidation test
		t.Run(fmt.Sprintf("random-invalidation-%d-limit-%d-entities-", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity){
				func(t *testing.T, backData *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
					testInvalidateAtRandom(t, list, entities)
				},
			}...)
		})
	}
}

// testInvalidatingHead keeps invalidating elements at random and evaluates whether the entities double linked-list remains
// connected from head to tail and reverse.
func testInvalidateAtRandom(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
	// total number of entities to store
	totalEntitiesStored := len(entities)
	// freeListInitialSize is total number of empty nodes after
	// storing all items in the list
	freeListInitialSize := len(list.values) - totalEntitiesStored

	// (i+1) keeps total invalidated (head) entities.
	for i := 0; i < totalEntitiesStored; i++ {
		list.invalidateRandomEntity()

		// size of list should be decremented after each invalidation.
		require.Equal(t, uint32(totalEntitiesStored-i-1), list.Size())

		// except when the list is empty, head and tail must be accessible after each invalidation
		// i.e., the linked list remains connected despite invalidation.
		if i != totalEntitiesStored-1 {
			// used list
			tailAccessibleFromHead(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			headAccessibleFromTail(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			// free list
			//
			// after invalidating each item, size of free list is incremented by one.
			headAccessibleFromTail(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+freeListInitialSize+1))

			tailAccessibleFromHead(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+freeListInitialSize+1))
		}
	}
}

// testInvalidatingHead keeps invalidating the head and evaluates the linked-list keeps updating its head
// and remains connected.
func testInvalidatingHead(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
	// total number of entities to store
	totalEntitiesStored := len(entities)
	// freeListInitialSize is total number of empty nodes after
	// storing all items in the list
	freeListInitialSize := len(list.values) - totalEntitiesStored

	// (i+1) keeps total invalidated (head) entities.
	for i := 0; i < totalEntitiesStored; i++ {
		headIndex := list.invalidateUsedHead()
		// head index should be moved to the next index after each head invalidation.
		require.Equal(t, EIndex(i), headIndex)
		// size of list should be decremented after each invalidation.
		require.Equal(t, uint32(totalEntitiesStored-i-1), list.Size())
		// invalidated head should be appended to free entities
		require.Equal(t, list.free.tail.sliceIndex(), headIndex)

		if freeListInitialSize != 0 {
			// number of entities is below limit, hence free list is not empty.
			// invalidating used head must not change the free head.
			require.Equal(t, EIndex(totalEntitiesStored), list.free.head.sliceIndex())
		} else {
			// number of entities is greater than or equal to limit, hence free list is empty.
			// free head must be updated to the first invalidated head (index 0),
			// and must be kept there for entire test (as we invalidate head not tail).
			require.Equal(t, EIndex(0), list.free.head.sliceIndex())
		}

		// except when the list is empty, head must be updated after invalidation,
		// except when the list is empty, head and tail must be accessible after each invalidation`
		// i.e., the linked list remains connected despite invalidation.
		if i != totalEntitiesStored-1 {
			// used linked list
			tailAccessibleFromHead(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			headAccessibleFromTail(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			// free list
			//
			// after invalidating each item, size of free list is incremented by one.
			tailAccessibleFromHead(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+1+freeListInitialSize))

			headAccessibleFromTail(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+1+freeListInitialSize))
		}

		// checking the status of head and tail in used list after each
		// head invalidation.
		usedTail, _ := list.getTails()
		usedHead, _ := list.getHeads()
		if i != totalEntitiesStored-1 {
			// list is not empty yet, we still have entities to invalidate.
			//
			// used tail should point to the last element in list, since we are
			// invalidating head.
			require.Equal(t, entities[totalEntitiesStored-1].ID(), usedTail.id)
			require.Equal(t, EIndex(totalEntitiesStored-1), list.used.tail.sliceIndex())

			// used head must point to the next element in the list,
			// i.e., invalidating head moves it forward.
			require.Equal(t, entities[i+1].ID(), usedHead.id)
			require.Equal(t, EIndex(i+1), list.used.head.sliceIndex())
		} else {
			// list is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, list.used.tail.isUndefined())
			require.True(t, list.used.head.isUndefined())
		}
	}
}

// testInvalidatingHead keeps invalidating the tail and evaluates the linked-list keeps updating its tail
// and remains connected.
func testInvalidatingTail(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity) {
	size := len(entities)
	offset := len(list.values) - size
	for i := 0; i < size; i++ {
		// invalidates tail index
		tailIndex := list.used.tail.sliceIndex()
		require.Equal(t, EIndex(size-1-i), tailIndex)

		list.invalidateEntityAtIndex(tailIndex)
		// old head index must be invalidated
		require.True(t, list.isInvalidated(tailIndex))
		// unclaimed head should be appended to free entities
		require.Equal(t, list.free.tail.sliceIndex(), tailIndex)

		if offset != 0 {
			// number of entities is below limit
			// free must head keeps pointing to first empty index after
			// adding all entities.
			require.Equal(t, EIndex(size), list.free.head.sliceIndex())
		} else {
			// number of entities is greater than or equal to limit
			// free head must be updated to last element in the list (size - 1),
			// and must be kept there for entire test (as we invalidate tail not head).
			require.Equal(t, EIndex(size-1), list.free.head.sliceIndex())
		}

		// size of list should be shrunk after each invalidation.
		require.Equal(t, uint32(size-i-1), list.Size())

		// except when the list is empty, tail must be updated after invalidation,
		// and also head and tail must be accessible after each invalidation
		// i.e., the linked list remains connected despite invalidation.
		if i != size-1 {

			// used list
			tailAccessibleFromHead(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			headAccessibleFromTail(t,
				list.used.head.sliceIndex(),
				list.used.tail.sliceIndex(),
				list,
				list.Size())

			// free list
			tailAccessibleFromHead(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+1+offset))

			headAccessibleFromTail(t,
				list.free.head.sliceIndex(),
				list.free.tail.sliceIndex(),
				list,
				uint32(i+1+offset))
		}

		usedTail, _ := list.getTails()
		usedHead, _ := list.getHeads()
		if i != size-1 {
			// list is not empty yet
			//
			// used tail should move backward after each invalidation
			require.Equal(t, entities[size-i-2].ID(), usedTail.id)
			require.Equal(t, EIndex(size-i-2), list.used.tail.sliceIndex())

			// used head must point to the first element in the list,
			require.Equal(t, entities[0].ID(), usedHead.id)
			require.Equal(t, EIndex(0), list.used.head.sliceIndex())
		} else {
			// list is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, list.used.tail.isUndefined())
			require.True(t, list.used.head.isUndefined())
		}
	}
}

// testInitialization evaluates the state of an initialized cachedEntity list before adding any element
// to it.
func testInitialization(t *testing.T, list *EntityDoubleLinkedList, _ []*unittest.MockEntity) {
	// head and tail of "used" linked-list must be undefined at initialization time, since
	// we have no elements in the list.
	require.True(t, list.used.head.isUndefined())
	require.True(t, list.used.tail.isUndefined())

	for i := 0; i < len(list.values); i++ {
		if i == 0 {
			// head of "free" linked-list should point to index 0 of entities slice.
			require.Equal(t, EIndex(i), list.free.head.sliceIndex())
			// previous element of head must be undefined (linked-list head feature).
			require.True(t, list.values[i].node.prev.isUndefined())
		}

		if i != 0 {
			// except head, any element should point back to its previous index in slice.
			require.Equal(t, EIndex(i-1), list.values[i].node.prev.sliceIndex())
		}

		if i != len(list.values)-1 {
			// except tail, any element should point forward to its next index in slice.
			require.Equal(t, EIndex(i+1), list.values[i].node.next.sliceIndex())
		}

		if i == len(list.values)-1 {
			// tail of "free" linked-list should point to the last index in entities slice.
			require.Equal(t, EIndex(i), list.free.tail.sliceIndex())
			// next element of tail must be undefined.
			require.True(t, list.values[i].node.next.isUndefined())
		}
	}
}

// testAddingEntities evaluates health of entities linked list for storing new elements.
func testAddingEntities(t *testing.T, list *EntityDoubleLinkedList, entitiesToBeAdded []*unittest.MockEntity, ejectionMode EjectionMode) {
	// adding elements
	for i, e := range entitiesToBeAdded {
		// adding each element must be successful.
		list.Add(e.ID(), e, uint64(i))

		if i < len(list.values) {
			// in case of no over limit, size of entities linked list should be incremented by each addition.
			require.Equal(t, list.Size(), uint32(i+1))
		}

		if ejectionMode == LRUEjection {
			// under LRU ejection mode, new entity should be placed at index i in back data
			_, entity, _ := list.Get(EIndex(i % len(list.values)))
			require.Equal(t, e, entity)
		}

		// linked-list sanity check
		// first insertion forward, head of used list should always point to
		// first entity in the list.
		usedHead, freeHead := list.getHeads()
		usedTail, freeTail := list.getTails()

		if ejectionMode == LRUEjection {
			expectedUsedHead := 0
			if i >= len(list.values) {
				// we are beyond limit, so LRU ejection must happen and used head must
				// be moved.
				expectedUsedHead = (i + 1) % len(list.values)
			}
			require.Equal(t, list.values[expectedUsedHead].entity, usedHead.entity)
			// head must be healthy and point back to undefined.
			require.True(t, usedHead.node.prev.isUndefined())
		}

		// new entity must be successfully added to tail of used list
		require.Equal(t, entitiesToBeAdded[i], usedTail.entity)
		// used tail must be healthy and point back to undefined.
		require.True(t, usedTail.node.next.isUndefined())

		// free head
		if i < len(list.values)-1 {
			// as long as we are below limit, after adding i element, free head
			// should move to i+1 element.
			require.Equal(t, EIndex(i+1), list.free.head.sliceIndex())
			// head must be healthy and point back to undefined.
			require.True(t, freeHead.node.prev.isUndefined())
		} else {
			// once we go beyond limit,
			// we run out of free slots,
			// and free head must be kept at undefined.
			require.Nil(t, freeHead)
		}

		// free tail
		if i < len(list.values)-1 {
			// as long as we are below limit, after adding i element, free tail
			// must keep pointing to last index of the array-based linked list. In other
			// words, adding element must not change free tail (since only free head is
			// updated).
			require.Equal(t, EIndex(len(list.values)-1), list.free.tail.sliceIndex())
			// head tail be healthy and point next to undefined.
			require.True(t, freeTail.node.next.isUndefined())
		} else {
			// once we go beyond limit, we run out of free slots, and
			// free tail must be kept at undefined.
			require.Nil(t, freeTail)
		}

		// used list
		// if we are still below limit, head to tail of used list
		// must be reachable within i + 1 steps.
		// +1 is since we start from index 0 not 1.
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

		// free list
		// if we are still below limit, head to tail of used list
		// must be reachable within "limit - i - 1" steps. "limit - i" part is since
		// when we have i elements in list, we have "limit - i" free slots, and -1 is
		// since we start from index 0 not 1.
		freeTraverseStep := uint32(len(list.values) - i - 1)
		if i >= len(list.values) {
			// if we are above the limit, head and tail of free list must be reachable
			// within 0 steps.
			// The reason is list is full and adding new elements is done
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

// testRetrievingEntitiesFrom evaluates that all entities starting from given index are retrievable from list.
func testRetrievingEntitiesFrom(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity, from EIndex) {
	for i := from; i < EIndex(len(entities)); i++ {
		actualID, actual, _ := list.Get(i % EIndex(len(list.values)))
		require.Equal(t, entities[i].ID(), actualID)
		require.Equal(t, entities[i], actual)
	}
}

// testRetrievingCount evaluates that exactly expected number of entities are retrievable from underlying list.
func testRetrievingCount(t *testing.T, list *EntityDoubleLinkedList, entities []*unittest.MockEntity, expected int) {
	actualRetrievable := 0

	for i := EIndex(0); i < EIndex(len(entities)); i++ {
		for j := EIndex(0); j < EIndex(len(list.values)); j++ {
			actualID, actual, _ := list.Get(j % EIndex(len(list.values)))
			if entities[i].ID() == actualID && entities[i] == actual {
				actualRetrievable++
			}
		}
	}

	require.Equal(t, expected, actualRetrievable)
}

// withTestScenario creates a new entity list, and then runs helpers
// on the entity list sequentially.
func withTestScenario(t *testing.T,
	limit uint32,
	entityCount uint32,
	ejectionMode EjectionMode,
	helpers ...func(*testing.T, *EntityDoubleLinkedList, []*unittest.MockEntity)) {

	list := NewEntityList(limit, ejectionMode)

	// head on underlying linked list value should be uninitialized
	require.True(t, list.used.head.isUndefined())
	require.Equal(t, list.Size(), uint32(0))

	entities := unittest.EntityListFixture(uint(entityCount))

	for _, helper := range helpers {
		helper(t, list, entities)
	}
}

// tailAccessibleFromHead checks tail of given entities linked list is reachable from its head by traversing expected number of steps.
func tailAccessibleFromHead(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, list *EntityDoubleLinkedList, steps uint32) {
	seen := make(map[EIndex]struct{})

	index := headSliceIndex
	for i := uint32(0); i < steps; i++ {
		if i == steps-1 {
			require.Equal(t, tailSliceIndex, index, "tail not reachable after steps steps")
			return
		}

		require.NotEqual(t, tailSliceIndex, index, "tail visited in less expected steps (potential inconsistency)", i, steps)
		_, ok := seen[index]
		require.False(t, ok, "duplicate identifiers found")

		require.False(t, list.values[index].node.next.isUndefined(), "tail not found, and reached end of list")
		index = list.values[index].node.next.sliceIndex()
	}
}

//  headAccessibleFromTail checks head of given entities linked list is reachable from its tail by traversing expected number of steps.
func headAccessibleFromTail(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, list *EntityDoubleLinkedList, total uint32) {
	seen := make(map[EIndex]struct{})

	index := tailSliceIndex
	for i := uint32(0); i < total; i++ {
		if i == total-1 {
			require.Equal(t, headSliceIndex, index, "head not reachable after total steps")
			return
		}

		require.NotEqual(t, headSliceIndex, index, "head visited in less expected steps (potential inconsistency)", i, total)
		_, ok := seen[index]
		require.False(t, ok, "duplicate identifiers found")

		index = list.values[index].node.prev.sliceIndex()
	}
}
