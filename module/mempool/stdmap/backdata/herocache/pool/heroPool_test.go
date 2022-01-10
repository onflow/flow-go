package pool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestStoreAndRetrieval_Without_Ejection checks health of heroPool for storing and retrieval scenarios that
// do not involve ejection.
// The test involves cases for testing the pool below its limit, and also up to its limit. However, it never gets beyond
// the limit, so no ejection will kick-in.
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testInitialization(t, pool, entities)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, pool, entities, LRUEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testRetrievingEntitiesFrom(t, pool, entities, 0)
				},
			}...,
			)
		})
	}
}

// TestStoreAndRetrieval_With_LRU_Ejection checks health of heroPool for storing and retrieval scenarios that involves the LRU ejection.
// The test involves cases for testing the pool beyond its limit, so the LRU ejection will kick-in.
func TestStoreAndRetrieval_With_LRU_Ejection(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of pool
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, pool, entities, LRUEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					// with a limit of tc.limit, storing a total of tc.entityCount (> tc.limit) entities, results
					// in ejection of the first tc.entityCount - tc.limit entities.
					// Hence, we check retrieval of the last tc.limit entities, which start from index
					// tc.entityCount - tc.limit entities.
					testRetrievingEntitiesFrom(t, pool, entities, EIndex(tc.entityCount-tc.limit))
				},
			}...,
			)
		})
	}
}

// TestStoreAndRetrieval_With_Random_Ejection checks health of heroPool for storing and retrieval scenarios that involves the LRU ejection.
func TestStoreAndRetrieval_With_Random_Ejection(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of pool
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
			withTestScenario(t, tc.limit, tc.entityCount, RandomEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, backData *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, RandomEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					// with a limit of tc.limit, storing a total of tc.entityCount (> tc.limit) entities, results
					// in ejection of "tc.entityCount - tc.limit" entities at random.
					// Hence, we check retrieval any successful total of "tc.limit" entities.
					testRetrievingCount(t, pool, entities, int(tc.limit))
				},
			}...,
			)
		})
	}
}

// TestInvalidateEntity checks the health of heroPool for invalidating entities under random, LRU, and LIFO scenarios.
// Invalidating an entity removes it from the used state and moves its node to the free state.
func TestInvalidateEntity(t *testing.T) {
	for _, tc := range []struct {
		limit       uint32 // capacity of entity pool
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, backData *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testInvalidatingHead(t, pool, entities)
				},
			}...)
		})

		// tail invalidation test (LIFO)
		t.Run(fmt.Sprintf("tail-invalidation-%d-limit-%d-entities-", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, backData *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testInvalidatingTail(t, pool, entities)
				},
			}...)
		})

		// random invalidation test
		t.Run(fmt.Sprintf("random-invalidation-%d-limit-%d-entities-", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *HeroPool, []*unittest.MockEntity){
				func(t *testing.T, backData *HeroPool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
					testInvalidateAtRandom(t, pool, entities)
				},
			}...)
		})
	}
}

// testInvalidatingHead keeps invalidating elements at random and evaluates whether the linked-lists keeping the used and free state remains connected
// from head to tail, and vice versa.
func testInvalidateAtRandom(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
	// total number of entities to store
	totalEntitiesStored := len(entities)
	// freeListInitialSize is total number of empty nodes after
	// storing all items in the list
	freeListInitialSize := len(pool.values) - totalEntitiesStored

	// (i+1) keeps total invalidated (head) entities.
	for i := 0; i < totalEntitiesStored; i++ {
		pool.invalidateRandomEntity()

		// size of pool should be decremented after each invalidation.
		require.Equal(t, uint32(totalEntitiesStored-i-1), pool.Size())

		// except when the pool is empty, head and tail must be accessible after each invalidation
		// i.e., the underlying linked list remains connected despite invalidation.
		if i != totalEntitiesStored-1 {
			// used linked-list
			tailAccessibleFromHead(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			headAccessibleFromTail(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			// free linked-list
			//
			// after invalidating each item, size of free linked-list is incremented by one.
			headAccessibleFromTail(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+freeListInitialSize+1))

			tailAccessibleFromHead(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+freeListInitialSize+1))
		}
	}
}

// testInvalidatingHead keeps invalidating the head and evaluates the linked-list keeps updating its head
// and remains connected.
func testInvalidatingHead(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
	// total number of entities to store
	totalEntitiesStored := len(entities)
	// freeListInitialSize is total number of empty nodes after
	// storing all items in the list
	freeListInitialSize := len(pool.values) - totalEntitiesStored

	// (i+1) keeps total invalidated (head) entities.
	for i := 0; i < totalEntitiesStored; i++ {
		headIndex := pool.invalidateUsedHead()
		// head index should be moved to the next index after each head invalidation.
		require.Equal(t, EIndex(i), headIndex)
		// size of list should be decremented after each invalidation.
		require.Equal(t, uint32(totalEntitiesStored-i-1), pool.Size())
		// invalidated head should be appended to free entities
		require.Equal(t, pool.free.tail.sliceIndex(), headIndex)

		if freeListInitialSize != 0 {
			// number of entities is below limit, hence free list is not empty.
			// invalidating used head must not change the free head.
			require.Equal(t, EIndex(totalEntitiesStored), pool.free.head.sliceIndex())
		} else {
			// number of entities is greater than or equal to limit, hence free list is empty.
			// free head must be updated to the first invalidated head (index 0),
			// and must be kept there for entire test (as we invalidate head not tail).
			require.Equal(t, EIndex(0), pool.free.head.sliceIndex())
		}

		// except when the list is empty, head must be updated after invalidation,
		// except when the list is empty, head and tail must be accessible after each invalidation`
		// i.e., the linked list remains connected despite invalidation.
		if i != totalEntitiesStored-1 {
			// used linked-list
			tailAccessibleFromHead(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			headAccessibleFromTail(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			// free lined-list
			//
			// after invalidating each item, size of free linked-list is incremented by one.
			tailAccessibleFromHead(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+1+freeListInitialSize))

			headAccessibleFromTail(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+1+freeListInitialSize))
		}

		// checking the status of head and tail in used linked-list after each head invalidation.
		usedTail, _ := pool.getTails()
		usedHead, _ := pool.getHeads()
		if i != totalEntitiesStored-1 {
			// pool is not empty yet, we still have entities to invalidate.
			//
			// used tail should point to the last element in pool, since we are
			// invalidating head.
			require.Equal(t, entities[totalEntitiesStored-1].ID(), usedTail.id)
			require.Equal(t, EIndex(totalEntitiesStored-1), pool.used.tail.sliceIndex())

			// used head must point to the next element in the pool,
			// i.e., invalidating head moves it forward.
			require.Equal(t, entities[i+1].ID(), usedHead.id)
			require.Equal(t, EIndex(i+1), pool.used.head.sliceIndex())
		} else {
			// pool is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, pool.used.tail.isUndefined())
			require.True(t, pool.used.head.isUndefined())
		}
	}
}

// testInvalidatingHead keeps invalidating the tail and evaluates the underlying free and used linked-lists keep updating its tail and remains connected.
func testInvalidatingTail(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity) {
	size := len(entities)
	offset := len(pool.values) - size
	for i := 0; i < size; i++ {
		// invalidates tail index
		tailIndex := pool.used.tail.sliceIndex()
		require.Equal(t, EIndex(size-1-i), tailIndex)

		pool.invalidateEntityAtIndex(tailIndex)
		// old head index must be invalidated
		require.True(t, pool.isInvalidated(tailIndex))
		// unclaimed head should be appended to free entities
		require.Equal(t, pool.free.tail.sliceIndex(), tailIndex)

		if offset != 0 {
			// number of entities is below limit
			// free must head keeps pointing to first empty index after
			// adding all entities.
			require.Equal(t, EIndex(size), pool.free.head.sliceIndex())
		} else {
			// number of entities is greater than or equal to limit
			// free head must be updated to last element in the pool (size - 1),
			// and must be kept there for entire test (as we invalidate tail not head).
			require.Equal(t, EIndex(size-1), pool.free.head.sliceIndex())
		}

		// size of pool should be shrunk after each invalidation.
		require.Equal(t, uint32(size-i-1), pool.Size())

		// except when the pool is empty, tail must be updated after invalidation,
		// and also head and tail must be accessible after each invalidation
		// i.e., the linked-list remains connected despite invalidation.
		if i != size-1 {

			// used linked-list
			tailAccessibleFromHead(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			headAccessibleFromTail(t,
				pool.used.head.sliceIndex(),
				pool.used.tail.sliceIndex(),
				pool,
				pool.Size())

			// free linked-list
			tailAccessibleFromHead(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+1+offset))

			headAccessibleFromTail(t,
				pool.free.head.sliceIndex(),
				pool.free.tail.sliceIndex(),
				pool,
				uint32(i+1+offset))
		}

		usedTail, _ := pool.getTails()
		usedHead, _ := pool.getHeads()
		if i != size-1 {
			// pool is not empty yet
			//
			// used tail should move backward after each invalidation
			require.Equal(t, entities[size-i-2].ID(), usedTail.id)
			require.Equal(t, EIndex(size-i-2), pool.used.tail.sliceIndex())

			// used head must point to the first element in the pool,
			require.Equal(t, entities[0].ID(), usedHead.id)
			require.Equal(t, EIndex(0), pool.used.head.sliceIndex())
		} else {
			// pool is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, pool.used.tail.isUndefined())
			require.True(t, pool.used.head.isUndefined())
		}
	}
}

// testInitialization evaluates the state of an initialized pool before adding any element to it.
func testInitialization(t *testing.T, pool *HeroPool, _ []*unittest.MockEntity) {
	// head and tail of "used" linked-list must be undefined at initialization time, since we have no elements in the list.
	require.True(t, pool.used.head.isUndefined())
	require.True(t, pool.used.tail.isUndefined())

	for i := 0; i < len(pool.values); i++ {
		if i == 0 {
			// head of "free" linked-list should point to index 0 of entities slice.
			require.Equal(t, EIndex(i), pool.free.head.sliceIndex())
			// previous element of head must be undefined (linked-list head feature).
			require.True(t, pool.values[i].node.prev.isUndefined())
		}

		if i != 0 {
			// except head, any element should point back to its previous index in slice.
			require.Equal(t, EIndex(i-1), pool.values[i].node.prev.sliceIndex())
		}

		if i != len(pool.values)-1 {
			// except tail, any element should point forward to its next index in slice.
			require.Equal(t, EIndex(i+1), pool.values[i].node.next.sliceIndex())
		}

		if i == len(pool.values)-1 {
			// tail of "free" linked-list should point to the last index in entities slice.
			require.Equal(t, EIndex(i), pool.free.tail.sliceIndex())
			// next element of tail must be undefined.
			require.True(t, pool.values[i].node.next.isUndefined())
		}
	}
}

// testAddingEntities evaluates health of pool for storing new elements.
func testAddingEntities(t *testing.T, pool *HeroPool, entitiesToBeAdded []*unittest.MockEntity, ejectionMode EjectionMode) {
	// adding elements
	for i, e := range entitiesToBeAdded {
		// adding each element must be successful.
		pool.Add(e.ID(), e, uint64(i))

		if i < len(pool.values) {
			// in case of no over limit, size of entities linked list should be incremented by each addition.
			require.Equal(t, pool.Size(), uint32(i+1))
		}

		if ejectionMode == LRUEjection {
			// under LRU ejection mode, new entity should be placed at index i in back data
			_, entity, _ := pool.Get(EIndex(i % len(pool.values)))
			require.Equal(t, e, entity)
		}

		// underlying linked-lists sanity check
		// first insertion forward, head of used list should always point to first entity in the list.
		usedHead, freeHead := pool.getHeads()
		usedTail, freeTail := pool.getTails()

		if ejectionMode == LRUEjection {
			expectedUsedHead := 0
			if i >= len(pool.values) {
				// we are beyond limit, so LRU ejection must happen and used head must
				// be moved.
				expectedUsedHead = (i + 1) % len(pool.values)
			}
			require.Equal(t, pool.values[expectedUsedHead].entity, usedHead.entity)
			// head must be healthy and point back to undefined.
			require.True(t, usedHead.node.prev.isUndefined())
		}

		// new entity must be successfully added to tail of used linked-list
		require.Equal(t, entitiesToBeAdded[i], usedTail.entity)
		// used tail must be healthy and point back to undefined.
		require.True(t, usedTail.node.next.isUndefined())

		// free head
		if i < len(pool.values)-1 {
			// as long as we are below limit, after adding i element, free head
			// should move to i+1 element.
			require.Equal(t, EIndex(i+1), pool.free.head.sliceIndex())
			// head must be healthy and point back to undefined.
			require.True(t, freeHead.node.prev.isUndefined())
		} else {
			// once we go beyond limit,
			// we run out of free slots,
			// and free head must be kept at undefined.
			require.Nil(t, freeHead)
		}

		// free tail
		if i < len(pool.values)-1 {
			// as long as we are below limit, after adding i element, free tail
			// must keep pointing to last index of the array-based linked-list. In other
			// words, adding element must not change free tail (since only free head is
			// updated).
			require.Equal(t, EIndex(len(pool.values)-1), pool.free.tail.sliceIndex())
			// head tail be healthy and point next to undefined.
			require.True(t, freeTail.node.next.isUndefined())
		} else {
			// once we go beyond limit, we run out of free slots, and
			// free tail must be kept at undefined.
			require.Nil(t, freeTail)
		}

		// used linked-list
		// if we are still below limit, head to tail of used linked-list
		// must be reachable within i + 1 steps.
		// +1 is since we start from index 0 not 1.
		usedTraverseStep := uint32(i + 1)
		if i >= len(pool.values) {
			// if we are above the limit, head to tail of used linked-list
			// must be reachable within as many steps as the actual capacity of pool.
			usedTraverseStep = uint32(len(pool.values))
		}
		tailAccessibleFromHead(t,
			pool.used.head.sliceIndex(),
			pool.used.tail.sliceIndex(),
			pool,
			usedTraverseStep)
		headAccessibleFromTail(t,
			pool.used.head.sliceIndex(),
			pool.used.tail.sliceIndex(),
			pool,
			usedTraverseStep)

		// free linked-list
		// if we are still below limit, head to tail of used linked-list
		// must be reachable within "limit - i - 1" steps. "limit - i" part is since
		// when we have i elements in pool, we have "limit - i" free slots, and -1 is
		// since we start from index 0 not 1.
		freeTraverseStep := uint32(len(pool.values) - i - 1)
		if i >= len(pool.values) {
			// if we are above the limit, head and tail of free linked-list must be reachable
			// within 0 steps.
			// The reason is linked-list is full and adding new elements is done
			// by ejecting existing ones, remaining no free slot.
			freeTraverseStep = uint32(0)
		}
		tailAccessibleFromHead(t,
			pool.free.head.sliceIndex(),
			pool.free.tail.sliceIndex(),
			pool,
			freeTraverseStep)
		headAccessibleFromTail(t,
			pool.free.head.sliceIndex(),
			pool.free.tail.sliceIndex(),
			pool,
			freeTraverseStep)
	}
}

// testRetrievingEntitiesFrom evaluates that all entities starting from given index are retrievable from pool.
func testRetrievingEntitiesFrom(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity, from EIndex) {
	for i := from; i < EIndex(len(entities)); i++ {
		actualID, actual, _ := pool.Get(i % EIndex(len(pool.values)))
		require.Equal(t, entities[i].ID(), actualID)
		require.Equal(t, entities[i], actual)
	}
}

// testRetrievingCount evaluates that exactly expected number of entities are retrievable from underlying pool.
func testRetrievingCount(t *testing.T, pool *HeroPool, entities []*unittest.MockEntity, expected int) {
	actualRetrievable := 0

	for i := EIndex(0); i < EIndex(len(entities)); i++ {
		for j := EIndex(0); j < EIndex(len(pool.values)); j++ {
			actualID, actual, _ := pool.Get(j % EIndex(len(pool.values)))
			if entities[i].ID() == actualID && entities[i] == actual {
				actualRetrievable++
			}
		}
	}

	require.Equal(t, expected, actualRetrievable)
}

// withTestScenario creates a new pool, and then runs helpers on it sequentially.
func withTestScenario(t *testing.T,
	limit uint32,
	entityCount uint32,
	ejectionMode EjectionMode,
	helpers ...func(*testing.T, *HeroPool, []*unittest.MockEntity)) {

	pool := NewPool(limit, ejectionMode)

	// head on underlying linked-list value should be uninitialized
	require.True(t, pool.used.head.isUndefined())
	require.Equal(t, pool.Size(), uint32(0))

	entities := unittest.EntityListFixture(uint(entityCount))

	for _, helper := range helpers {
		helper(t, pool, entities)
	}
}

// tailAccessibleFromHead checks tail of given entities linked-list is reachable from its head by traversing expected number of steps.
func tailAccessibleFromHead(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, pool *HeroPool, steps uint32) {
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

		require.False(t, pool.values[index].node.next.isUndefined(), "tail not found, and reached end of list")
		index = pool.values[index].node.next.sliceIndex()
	}
}

//  headAccessibleFromTail checks head of given entities linked list is reachable from its tail by traversing expected number of steps.
func headAccessibleFromTail(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, pool *HeroPool, total uint32) {
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

		index = pool.values[index].node.prev.sliceIndex()
	}
}
