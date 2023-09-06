package heropool

import (
	"fmt"
	"math"
	"testing"

	"github.com/onflow/flow-go/utils/rand"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestStoreAndRetrieval_BelowLimit checks health of heroPool for storing and retrieval scenarios that
// do not involve ejection.
// The test involves cases for testing the pool below its limit, and also up to its limit. However, it never gets beyond
// the limit, so no ejection will kick-in.
func TestStoreAndRetrieval_BelowLimit(t *testing.T) {
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testInitialization(t, pool, entities)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, pool, entities, LRUEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testRetrievingEntitiesFrom(t, pool, entities, 0)
				},
			}...,
			)
		})
	}
}

// TestStoreAndRetrieval_With_No_Ejection checks health of heroPool for storing and retrieval scenarios that involves the NoEjection mode.
func TestStoreAndRetrieval_With_No_Ejection(t *testing.T) {
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
			withTestScenario(t, tc.limit, tc.entityCount, NoEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, pool, entities, NoEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					// with the NoEjection mode, only the first "limit" entities must be retrievable.
					testRetrievingEntitiesInRange(t, pool, entities, 0, EIndex(tc.limit))
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, pool, entities, LRUEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
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
			withTestScenario(t, tc.limit, tc.entityCount, RandomEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, backData *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, RandomEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
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
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, backData *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testInvalidatingHead(t, pool, entities)
				},
			}...)
		})

		// tail invalidation test (LIFO)
		t.Run(fmt.Sprintf("tail-invalidation-%d-limit-%d-entities-", tc.limit, tc.entityCount), func(t *testing.T) {
			withTestScenario(t, tc.limit, tc.entityCount, LRUEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
				func(t *testing.T, backData *Pool, entities []*unittest.MockEntity) {
					testAddingEntities(t, backData, entities, LRUEjection)
				},
				func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
					testInvalidatingTail(t, pool, entities)
				},
			}...)
		})
	}
}

// TestAddAndRemoveEntities checks health of heroPool for scenario where entitites are stored and removed in a predetermined order.
// LRUEjection, NoEjection and RandomEjection are tested. RandomEjection doesn't allow to provide a final state of the pool to check.
func TestAddAndRemoveEntities(t *testing.T) {
	for _, tc := range []struct {
		limit               uint32       // capacity of the pool
		entityCount         uint32       // total entities to be stored
		ejectionMode        EjectionMode // ejection mode
		numberOfOperations  int
		probabilityOfAdding float32
	}{
		{
			limit:               500,
			entityCount:         1000,
			ejectionMode:        LRUEjection,
			numberOfOperations:  1000,
			probabilityOfAdding: 0.8,
		},
		{
			limit:               500,
			entityCount:         1000,
			ejectionMode:        NoEjection,
			numberOfOperations:  1000,
			probabilityOfAdding: 0.8,
		},
		{
			limit:               500,
			entityCount:         1000,
			ejectionMode:        RandomEjection,
			numberOfOperations:  1000,
			probabilityOfAdding: 0.8,
		},
	} {
		t.Run(fmt.Sprintf("%d-limit-%d-entities", tc.limit, tc.entityCount), func(t *testing.T) {
			testAddRemoveEntities(t, tc.limit, tc.entityCount, tc.ejectionMode, tc.numberOfOperations, tc.probabilityOfAdding)
		})
	}
}

// testAddRemoveEntities adds and removes randomly elements in the pool, probabilityOfAdding and its counterpart 1-probabilityOfAdding are probabilities
// for an operation to be add or remove. Current timestamp is taken as a seed for the random number generator.
func testAddRemoveEntities(t *testing.T, limit uint32, entityCount uint32, ejectionMode EjectionMode, numberOfOperations int, probabilityOfAdding float32) {

	require.GreaterOrEqual(t, entityCount, 2*limit, "entityCount must be greater or equal to 2*limit to test add/remove operations")

	randomIntN := func(length int) int {
		random, err := rand.Uintn(uint(length))
		require.NoError(t, err)
		return int(random)
	}

	pool := NewHeroPool(limit, ejectionMode, unittest.Logger())
	entities := unittest.EntityListFixture(uint(entityCount))
	// retryLimit is the max number of retries to find an entity that is not already in the pool to add it.
	// The test fails if it reaches this limit.
	retryLimit := 100
	// an array of random owner Ids.
	ownerIds := make([]uint64, entityCount)
	// generate ownerId to index in the entities array.
	for i := 0; i < int(entityCount); i++ {
		randomOwnerId, err := rand.Uint64()
		require.Nil(t, err)
		ownerIds[i] = randomOwnerId
	}
	// this map maintains entities currently stored in the pool.
	addedEntities := make(map[flow.Identifier]int)
	addedEntitiesInPool := make(map[flow.Identifier]EIndex)
	for i := 0; i < numberOfOperations; i++ {
		// choose between Add and Remove with a probability of probabilityOfAdding and 1-probabilityOfAdding respectively.
		if float32(randomIntN(math.MaxInt32))/math.MaxInt32 < probabilityOfAdding || len(addedEntities) == 0 {
			// keeps finding an entity to add until it finds one that is not already in the pool.
			found := false
			for retryTime := 0; retryTime < retryLimit; retryTime++ {
				toAddIndex := randomIntN(int(entityCount))
				_, found = addedEntities[entities[toAddIndex].ID()]
				if !found {
					// found an entity that is not in the pool, add it.
					indexInThePool, _, ejectedEntity := pool.Add(entities[toAddIndex].ID(), entities[toAddIndex], ownerIds[toAddIndex])
					if ejectionMode != NoEjection || len(addedEntities) < int(limit) {
						// when there is an ejection mode in place, or the pool is not full, the index should be valid.
						require.NotEqual(t, InvalidIndex, indexInThePool)
					}
					require.LessOrEqual(t, len(addedEntities), int(limit), "pool should not contain more elements than its limit")
					if ejectionMode != NoEjection && len(addedEntities) == int(limit) {
						// when there is an ejection mode in place, the ejected entity should be valid.
						require.NotNil(t, ejectedEntity)
					}
					if ejectionMode != NoEjection && len(addedEntities) >= int(limit) {
						// when there is an ejection mode in place, the ejected entity should be valid.
						require.NotNil(t, ejectedEntity)
					}
					if indexInThePool != InvalidIndex {
						entityId := entities[toAddIndex].ID()
						// tracks the index of the entity in the pool and the index of the entity in the entities array.
						addedEntities[entityId] = int(toAddIndex)
						addedEntitiesInPool[entityId] = indexInThePool
						// any entity added to the pool should be in the pool, and must be retrievable.
						actualFlowId, actualEntity, actualOwnerId := pool.Get(indexInThePool)
						require.Equal(t, entityId, actualFlowId)
						require.Equal(t, entities[toAddIndex], actualEntity, "pool returned a different entity than the one added")
						require.Equal(t, ownerIds[toAddIndex], actualOwnerId, "pool returned a different owner than the one added")
					}
					if ejectedEntity != nil {
						require.Contains(t, addedEntities, ejectedEntity.ID(), "pool ejected an entity that was not added before")
						delete(addedEntities, ejectedEntity.ID())
						delete(addedEntitiesInPool, ejectedEntity.ID())
					}
					break
				}
			}
			require.Falsef(t, found, "could not find an entity to add after %d retries", retryLimit)
		} else {
			// randomly select an index of an entity to remove.
			entityToRemove := randomIntN(len(addedEntities))
			i := 0
			var indexInPoolToRemove EIndex = 0
			var indexInEntitiesArray int = 0
			for k, v := range addedEntities {
				if i == entityToRemove {
					indexInPoolToRemove = addedEntitiesInPool[k]
					indexInEntitiesArray = v
					break
				}
				i++
			}
			// remove the selected entity from the pool.
			removedEntity := pool.Remove(indexInPoolToRemove)
			expectedRemovedEntityId := entities[indexInEntitiesArray].ID()
			require.Equal(t, expectedRemovedEntityId, removedEntity.ID(), "removed wrong entity")
			delete(addedEntities, expectedRemovedEntityId)
			delete(addedEntitiesInPool, expectedRemovedEntityId)
			actualFlowId, actualEntity, _ := pool.Get(indexInPoolToRemove)
			require.Equal(t, flow.ZeroID, actualFlowId)
			require.Equal(t, nil, actualEntity)
		}
	}
	for k, v := range addedEntities {
		indexInPool := addedEntitiesInPool[k]
		actualFlowId, actualEntity, actualOwnerId := pool.Get(indexInPool)
		require.Equal(t, entities[v].ID(), actualFlowId)
		require.Equal(t, entities[v], actualEntity)
		require.Equal(t, ownerIds[v], actualOwnerId)
	}
	require.Equalf(t, len(addedEntities), int(pool.Size()), "pool size is not correct, expected %d, actual %d", len(addedEntities), pool.Size())
}

// testInvalidatingHead keeps invalidating the head and evaluates the linked-list keeps updating its head
// and remains connected.
func testInvalidatingHead(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
	// total number of entities to store
	totalEntitiesStored := len(entities)
	// freeListInitialSize is total number of empty nodes after
	// storing all items in the list
	freeListInitialSize := len(pool.poolEntities) - totalEntitiesStored

	// (i+1) keeps total invalidated (head) entities.
	for i := 0; i < totalEntitiesStored; i++ {
		headIndex := pool.invalidateUsedHead()
		// head index should be moved to the next index after each head invalidation.
		require.Equal(t, entities[i], headIndex)
		// size of list should be decremented after each invalidation.
		require.Equal(t, uint32(totalEntitiesStored-i-1), pool.Size())
		// invalidated head should be appended to free entities
		require.Equal(t, pool.states[stateFree].tail, EIndex(i))

		if freeListInitialSize != 0 {
			// number of entities is below limit, hence free list is not empty.
			// invalidating used head must not change the free head.
			require.Equal(t, EIndex(totalEntitiesStored), pool.states[stateFree].head)
		} else {
			// number of entities is greater than or equal to limit, hence free list is empty.
			// free head must be updated to the first invalidated head (index 0),
			// and must be kept there for entire test (as we invalidate head not tail).
			require.Equal(t, EIndex(0), pool.states[stateFree].head)
		}

		// except when the list is empty, head must be updated after invalidation,
		// except when the list is empty, head and tail must be accessible after each invalidation`
		// i.e., the linked list remains connected despite invalidation.
		if i != totalEntitiesStored-1 {
			// used linked-list
			tailAccessibleFromHead(t,
				pool.states[stateUsed].head,
				pool.states[stateUsed].tail,
				pool,
				pool.Size())

			headAccessibleFromTail(t,
				pool.states[stateUsed].head,
				pool.states[stateUsed].tail,
				pool,
				pool.Size())

			// free lined-list
			//
			// after invalidating each item, size of free linked-list is incremented by one.
			tailAccessibleFromHead(t,
				pool.states[stateFree].head,
				pool.states[stateFree].tail,
				pool,
				uint32(i+1+freeListInitialSize))

			headAccessibleFromTail(t,
				pool.states[stateFree].head,
				pool.states[stateFree].tail,
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
			require.Equal(t, EIndex(totalEntitiesStored-1), pool.states[stateUsed].tail)

			// used head must point to the next element in the pool,
			// i.e., invalidating head moves it forward.
			require.Equal(t, entities[i+1].ID(), usedHead.id)
			require.Equal(t, EIndex(i+1), pool.states[stateUsed].head)
		} else {
			// pool is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, pool.states[stateUsed].size == 0)
			require.Equal(t, pool.states[stateUsed].tail, InvalidIndex)
			require.Equal(t, pool.states[stateUsed].head, InvalidIndex)
		}
		checkEachEntityIsInFreeOrUsedState(t, pool)
	}
}

// testInvalidatingHead keeps invalidating the tail and evaluates the underlying free and used linked-lists keep updating its tail and remains connected.
func testInvalidatingTail(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
	size := len(entities)
	offset := len(pool.poolEntities) - size
	for i := 0; i < size; i++ {
		// invalidates tail index
		tailIndex := pool.states[stateUsed].tail
		require.Equal(t, EIndex(size-1-i), tailIndex)

		pool.invalidateEntityAtIndex(tailIndex)
		// old head index must be invalidated
		require.True(t, pool.isInvalidated(tailIndex))
		// unclaimed head should be appended to free entities
		require.Equal(t, pool.states[stateFree].tail, tailIndex)

		if offset != 0 {
			// number of entities is below limit
			// free must head keeps pointing to first empty index after
			// adding all entities.
			require.Equal(t, EIndex(size), pool.states[stateFree].head)
		} else {
			// number of entities is greater than or equal to limit
			// free head must be updated to last element in the pool (size - 1),
			// and must be kept there for entire test (as we invalidate tail not head).
			require.Equal(t, EIndex(size-1), pool.states[stateFree].head)
		}

		// size of pool should be shrunk after each invalidation.
		require.Equal(t, uint32(size-i-1), pool.Size())

		// except when the pool is empty, tail must be updated after invalidation,
		// and also head and tail must be accessible after each invalidation
		// i.e., the linked-list remains connected despite invalidation.
		if i != size-1 {

			// used linked-list
			tailAccessibleFromHead(t,
				pool.states[stateUsed].head,
				pool.states[stateUsed].tail,
				pool,
				pool.Size())

			headAccessibleFromTail(t,
				pool.states[stateUsed].head,
				pool.states[stateUsed].tail,
				pool,
				pool.Size())

			// free linked-list
			tailAccessibleFromHead(t,
				pool.states[stateFree].head,
				pool.states[stateFree].tail,
				pool,
				uint32(i+1+offset))

			headAccessibleFromTail(t,
				pool.states[stateFree].head,
				pool.states[stateFree].tail,
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
			require.Equal(t, EIndex(size-i-2), pool.states[stateUsed].tail)

			// used head must point to the first element in the pool,
			require.Equal(t, entities[0].ID(), usedHead.id)
			require.Equal(t, EIndex(0), pool.states[stateUsed].head)
		} else {
			// pool is empty
			// used head and tail must be nil and their corresponding
			// pointer indices must be undefined.
			require.Nil(t, usedHead)
			require.Nil(t, usedTail)
			require.True(t, pool.states[stateUsed].size == 0)
			require.Equal(t, pool.states[stateUsed].head, InvalidIndex)
			require.Equal(t, pool.states[stateUsed].tail, InvalidIndex)
		}
		checkEachEntityIsInFreeOrUsedState(t, pool)
	}
}

// testInitialization evaluates the state of an initialized pool before adding any element to it.
func testInitialization(t *testing.T, pool *Pool, _ []*unittest.MockEntity) {
	// "used" linked-list must have a zero size, since we have no elements in the list.
	require.True(t, pool.states[stateUsed].size == 0)
	require.Equal(t, pool.states[stateUsed].head, InvalidIndex)
	require.Equal(t, pool.states[stateUsed].tail, InvalidIndex)

	for i := 0; i < len(pool.poolEntities); i++ {
		if i == 0 {
			// head of "free" linked-list should point to InvalidIndex of entities slice.
			require.Equal(t, EIndex(i), pool.states[stateFree].head)
			// previous element of head must be undefined (linked-list head feature).
			require.Equal(t, pool.poolEntities[i].node.prev, InvalidIndex)
		}

		if i != 0 {
			// except head, any element should point back to its previous index in slice.
			require.Equal(t, EIndex(i-1), pool.poolEntities[i].node.prev)
		}

		if i != len(pool.poolEntities)-1 {
			// except tail, any element should point forward to its next index in slice.
			require.Equal(t, EIndex(i+1), pool.poolEntities[i].node.next)
		}

		if i == len(pool.poolEntities)-1 {
			// tail of "free" linked-list should point to the last index in entities slice.
			require.Equal(t, EIndex(i), pool.states[stateFree].tail)
			// next element of tail must be undefined.
			require.Equal(t, pool.poolEntities[i].node.next, InvalidIndex)
		}
	}
}

// testAddingEntities evaluates health of pool for storing new elements.
func testAddingEntities(t *testing.T, pool *Pool, entitiesToBeAdded []*unittest.MockEntity, ejectionMode EjectionMode) {
	// initially head must be empty
	e, ok := pool.Head()
	require.False(t, ok)
	require.Nil(t, e)

	var uniqueEntities map[flow.Identifier]struct{}
	if ejectionMode != NoEjection {
		uniqueEntities = make(map[flow.Identifier]struct{})
		for _, entity := range entitiesToBeAdded {
			uniqueEntities[entity.ID()] = struct{}{}
		}
		require.Equalf(t, len(uniqueEntities), len(entitiesToBeAdded), "entitesToBeAdded must be constructed of unique entities")
	}

	// adding elements
	lruEjectedIndex := 0
	for i, e := range entitiesToBeAdded {
		// adding each element must be successful.
		entityIndex, slotAvailable, ejectedEntity := pool.Add(e.ID(), e, uint64(i))

		if i < len(pool.poolEntities) {
			// in case of no over limit, size of entities linked list should be incremented by each addition.
			require.Equal(t, pool.Size(), uint32(i+1))

			require.True(t, slotAvailable)
			require.Nil(t, ejectedEntity)
			require.Equal(t, entityIndex, EIndex(i))

			// in case pool is not full, the head should retrieve the first added entity.
			headEntity, headExists := pool.Head()
			require.True(t, headExists)
			require.Equal(t, headEntity.ID(), entitiesToBeAdded[0].ID())
		}

		if ejectionMode == LRUEjection {
			// under LRU ejection mode, new entity should be placed at index i in back data
			_, entity, _ := pool.Get(EIndex(i % len(pool.poolEntities)))
			require.Equal(t, e, entity)

			if i >= len(pool.poolEntities) {
				require.True(t, slotAvailable)
				require.NotNil(t, ejectedEntity)
				// confirm that ejected entity is the oldest entity
				require.Equal(t, entitiesToBeAdded[lruEjectedIndex], ejectedEntity)
				lruEjectedIndex++
				// when pool is full and with LRU ejection, the head should move forward with each element added.
				headEntity, headExists := pool.Head()
				require.True(t, headExists)
				require.Equal(t, headEntity.ID(), entitiesToBeAdded[i+1-len(pool.poolEntities)].ID())
			}
		}

		if ejectionMode == RandomEjection {
			if i >= len(pool.poolEntities) {
				require.True(t, slotAvailable)
				require.NotNil(t, ejectedEntity)
				// confirm that ejected entity is from list of entitiesToBeAdded
				_, ok := uniqueEntities[ejectedEntity.ID()]
				require.True(t, ok)
			}
		}

		if ejectionMode == NoEjection {
			if i >= len(pool.poolEntities) {
				require.False(t, slotAvailable)
				require.Nil(t, ejectedEntity)
				require.Equal(t, entityIndex, InvalidIndex)

				// when pool is full and with NoEjection, the head must keep pointing to the first added element.
				headEntity, headExists := pool.Head()
				require.True(t, headExists)
				require.Equal(t, headEntity.ID(), entitiesToBeAdded[0].ID())
			}
		}

		// underlying linked-lists sanity check
		// first insertion forward, head of used list should always point to first entity in the list.
		usedHead, freeHead := pool.getHeads()
		usedTail, freeTail := pool.getTails()

		if ejectionMode == LRUEjection {
			expectedUsedHead := 0
			if i >= len(pool.poolEntities) {
				// we are beyond limit, so LRU ejection must happen and used head must
				// be moved.
				expectedUsedHead = (i + 1) % len(pool.poolEntities)
			}
			require.Equal(t, pool.poolEntities[expectedUsedHead].entity, usedHead.entity)
			// head must be healthy and point back to undefined.
			require.Equal(t, usedHead.node.prev, InvalidIndex)
		}

		if ejectionMode != NoEjection || i < len(pool.poolEntities) {
			// new entity must be successfully added to tail of used linked-list
			require.Equal(t, entitiesToBeAdded[i], usedTail.entity)
			// used tail must be healthy and point back to undefined.
			require.Equal(t, usedTail.node.next, InvalidIndex)
		}

		if ejectionMode == NoEjection && i >= len(pool.poolEntities) {
			// used tail must not move
			require.Equal(t, entitiesToBeAdded[len(pool.poolEntities)-1], usedTail.entity)
			// used tail must be healthy and point back to undefined.
			// This is not needed anymore as tail's next is now ignored
			require.Equal(t, usedTail.node.next, InvalidIndex)
		}

		// free head
		if i < len(pool.poolEntities)-1 {
			// as long as we are below limit, after adding i element, free head
			// should move to i+1 element.
			require.Equal(t, EIndex(i+1), pool.states[stateFree].head)
			// head must be healthy and point back to undefined.
			require.Equal(t, freeHead.node.prev, InvalidIndex)
		} else {
			// once we go beyond limit,
			// we run out of free slots,
			// and free head must be kept at undefined.
			require.Nil(t, freeHead)
		}

		// free tail
		if i < len(pool.poolEntities)-1 {
			// as long as we are below limit, after adding i element, free tail
			// must keep pointing to last index of the array-based linked-list. In other
			// words, adding element must not change free tail (since only free head is
			// updated).
			require.Equal(t, EIndex(len(pool.poolEntities)-1), pool.states[stateFree].tail)
			// head tail be healthy and point next to undefined.
			require.Equal(t, freeTail.node.next, InvalidIndex)
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
		if i >= len(pool.poolEntities) {
			// if we are above the limit, head to tail of used linked-list
			// must be reachable within as many steps as the actual capacity of pool.
			usedTraverseStep = uint32(len(pool.poolEntities))
		}
		tailAccessibleFromHead(t,
			pool.states[stateUsed].head,
			pool.states[stateUsed].tail,
			pool,
			usedTraverseStep)
		headAccessibleFromTail(t,
			pool.states[stateUsed].head,
			pool.states[stateUsed].tail,
			pool,
			usedTraverseStep)

		// free linked-list
		// if we are still below limit, head to tail of used linked-list
		// must be reachable within "limit - i - 1" steps. "limit - i" part is since
		// when we have i elements in pool, we have "limit - i" free slots, and -1 is
		// since we start from index 0 not 1.
		freeTraverseStep := uint32(len(pool.poolEntities) - i - 1)
		if i >= len(pool.poolEntities) {
			// if we are above the limit, head and tail of free linked-list must be reachable
			// within 0 steps.
			// The reason is linked-list is full and adding new elements is done
			// by ejecting existing ones, remaining no free slot.
			freeTraverseStep = uint32(0)
		}
		tailAccessibleFromHead(t,
			pool.states[stateFree].head,
			pool.states[stateFree].tail,
			pool,
			freeTraverseStep)
		headAccessibleFromTail(t,
			pool.states[stateFree].head,
			pool.states[stateFree].tail,
			pool,
			freeTraverseStep)

		checkEachEntityIsInFreeOrUsedState(t, pool)
	}
}

// testRetrievingEntitiesFrom evaluates that all entities starting from given index are retrievable from pool.
func testRetrievingEntitiesFrom(t *testing.T, pool *Pool, entities []*unittest.MockEntity, from EIndex) {
	testRetrievingEntitiesInRange(t, pool, entities, from, EIndex(len(entities)))
}

// testRetrievingEntitiesInRange evaluates that all entities in the given range are retrievable from pool.
func testRetrievingEntitiesInRange(t *testing.T, pool *Pool, entities []*unittest.MockEntity, from EIndex, to EIndex) {
	for i := from; i < to; i++ {
		actualID, actual, _ := pool.Get(i % EIndex(len(pool.poolEntities)))
		require.Equal(t, entities[i].ID(), actualID, i)
		require.Equal(t, entities[i], actual, i)
	}
}

// testRetrievingCount evaluates that exactly expected number of entities are retrievable from underlying pool.
func testRetrievingCount(t *testing.T, pool *Pool, entities []*unittest.MockEntity, expected int) {
	actualRetrievable := 0

	for i := EIndex(0); i < EIndex(len(entities)); i++ {
		for j := EIndex(0); j < EIndex(len(pool.poolEntities)); j++ {
			actualID, actual, _ := pool.Get(j % EIndex(len(pool.poolEntities)))
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
	helpers ...func(*testing.T, *Pool, []*unittest.MockEntity)) {

	pool := NewHeroPool(limit, ejectionMode, unittest.Logger())

	// head on underlying linked-list value should be uninitialized
	require.True(t, pool.states[stateUsed].size == 0)
	require.Equal(t, pool.Size(), uint32(0))

	entities := unittest.EntityListFixture(uint(entityCount))

	for _, helper := range helpers {
		helper(t, pool, entities)
	}
}

// tailAccessibleFromHead checks tail of given entities linked-list is reachable from its head by traversing expected number of steps.
func tailAccessibleFromHead(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, pool *Pool, steps uint32) {
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

		require.NotEqual(t, pool.poolEntities[index].node.next, InvalidIndex, "tail not found, and reached end of list")
		index = pool.poolEntities[index].node.next
	}
}

// headAccessibleFromTail checks head of given entities linked list is reachable from its tail by traversing expected number of steps.
func headAccessibleFromTail(t *testing.T, headSliceIndex EIndex, tailSliceIndex EIndex, pool *Pool, total uint32) {
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

		index = pool.poolEntities[index].node.prev
	}
}

// checkEachEntityIsInFreeOrUsedState checks if each entity in the pool belongs exactly to one of the state lists.
func checkEachEntityIsInFreeOrUsedState(t *testing.T, pool *Pool) {
	pool_capacity := len(pool.poolEntities)
	// check size
	require.Equal(t, int(pool.states[stateFree].size+pool.states[stateUsed].size), pool_capacity, "Pool capacity is not equal to the sum of used and free sizes")
	// check elelments
	nodesInFree := discoverEntitiesBelongingToStateList(t, pool, stateFree)
	nodesInUsed := discoverEntitiesBelongingToStateList(t, pool, stateUsed)
	for i := 0; i < pool_capacity; i++ {
		require.False(t, !nodesInFree[i] && !nodesInUsed[i], "Node is not in any state list")
		require.False(t, nodesInFree[i] && nodesInUsed[i], "Node is in two state lists at the same time")
	}
}

// discoverEntitiesBelongingToStateList discovers all entities in the pool that belong to the given list.
func discoverEntitiesBelongingToStateList(t *testing.T, pool *Pool, stateType StateIndex) []bool {
	result := make([]bool, len(pool.poolEntities))
	for node_index := pool.states[stateType].head; node_index != InvalidIndex; {
		require.False(t, result[node_index], "A node is present two times in the same state list")
		result[node_index] = true
		node_index = pool.poolEntities[node_index].node.next
	}
	return result
}
