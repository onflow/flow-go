package herocache

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestArrayBackData_SingleBucket evaluates health of state transition for storing 10 entities in a Cache with only
// a single bucket (of 16). It also evaluates all stored items are retrievable.
func TestArrayBackData_SingleBucket(t *testing.T) {
	limit := 16

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		1,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// sanity checks
	for i := heropool.EIndex(0); i < heropool.EIndex(len(entities)); i++ {
		// since we are below limit, elements should be added sequentially at bucket 0.
		// the ith added element has a key index of i+1,
		// since 0 means unused key index in implementation.
		require.Equal(t, bd.buckets[0].slots[i].slotAge, uint64(i+1))
		// also, since we have not yet over-limited,
		// entities are assigned their entityIndex in the same order they are added.
		require.Equal(t, bd.buckets[0].slots[i].valueIndex, i)
		_, _, owner := bd.entities.Get(i)
		require.Equal(t, owner, uint64(i))
	}

	// all stored items must be retrievable
	testRetrievableFrom(t, bd, entities, 0)
}

// TestArrayBackData_Adjust evaluates that Adjust method correctly updates the value of
// the desired entity while preserving the integrity of BackData.
func TestArrayBackData_Adjust(t *testing.T) {
	limit := 100_000

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// picks a random entity from BackData and adjusts its identifier to a new one.
	entityIndex := rand.Int() % limit
	// checking integrity of retrieving entity
	oldEntity, ok := bd.Get(entities[entityIndex].ID())
	require.True(t, ok)
	oldEntityID := oldEntity.ID()
	require.Equal(t, entities[entityIndex].ID(), oldEntityID)
	require.Equal(t, entities[entityIndex], oldEntity)

	// picks a new identifier for the entity and makes sure it is different than its current one.
	newEntityID := unittest.IdentifierFixture()
	require.NotEqual(t, oldEntityID, newEntityID)

	// adjusts old entity to a new entity with a new identifier
	newEntity, ok := bd.Adjust(oldEntity.ID(), func(entity *unittest.MockEntity) *unittest.MockEntity {
		require.True(t, ok)
		// oldEntity must be passed to func parameter of adjust.
		require.Equal(t, oldEntityID, entity.ID())
		require.Equal(t, oldEntity, entity)

		return &unittest.MockEntity{Identifier: oldEntityID, Nonce: entity.Nonce + 1}
	})

	// adjustment must be successful, and identifier must be same.
	require.True(t, ok)
	require.Equal(t, oldEntityID, newEntity.ID())

	// replaces new entity in the original reference list and
	// retrieves all.
	entities[entityIndex] = newEntity
	testRetrievableFrom(t, bd, entities, 0)

	// re-adjusting the entity should succeed because the adjusted entity remains under the original id.
	entity, ok := bd.Adjust(oldEntityID, func(entity *unittest.MockEntity) *unittest.MockEntity {
		return &unittest.MockEntity{
			Identifier: entity.ID(), // preserve the old id
			Nonce:      entity.Nonce + 1,
		}
	})
	require.True(t, ok)
	require.NotNil(t, entity)

	// similarly, retrieving old entity must not fail
	entity, ok = bd.Get(oldEntityID)
	require.True(t, ok)
	require.NotNil(t, entity)

	ok = bd.Has(oldEntityID)
	require.True(t, ok)

	// adjusting any random non-existing identifier must fail
	entity, ok = bd.Adjust(unittest.IdentifierFixture(), func(entity *unittest.MockEntity) *unittest.MockEntity {
		require.Fail(t, "function must not be invoked on a non-existing entity")
		return entity
	})
	require.False(t, ok)
	require.Nil(t, entity)

	// adjustment must be idempotent for size
	require.Equal(t, bd.Size(), uint(limit))
}

// TestArrayBackData_AdjustWitInit evaluates that AdjustWithInit method. It should initialize and then adjust the value of
// non-existing entity while preserving the integrity of BackData on just adjusting the value of existing entity.
func TestArrayBackData_AdjustWitInit(t *testing.T) {
	limit := 100_000

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))
	for _, e := range entities {
		adjustedEntity, adjusted := bd.AdjustWithInit(e.ID(), func(entity *unittest.MockEntity) *unittest.MockEntity {
			// adjust logic, increments the nonce of the entity
			entity.Nonce++
			return entity
		}, func() *unittest.MockEntity {
			return e // initialize with the entity
		})
		require.True(t, adjusted)
		require.Equal(t, e.ID(), adjustedEntity.ID())
		require.Equal(t, uint64(1), adjustedEntity.Nonce)
	}

	// picks a random entity from BackData and adjusts its identifier to a new one.
	entityIndex := rand.Int() % limit
	// checking integrity of retrieving entity
	oldEntity, ok := bd.Get(entities[entityIndex].ID())
	require.True(t, ok)
	oldEntityID := oldEntity.ID()
	require.Equal(t, entities[entityIndex].ID(), oldEntityID)
	require.Equal(t, entities[entityIndex], oldEntity)

	// picks a new identifier for the entity and makes sure it is different than its current one.
	newEntityID := unittest.IdentifierFixture()
	require.NotEqual(t, oldEntityID, newEntityID)

	// adjusts old entity to a new entity with a new identifier
	newEntity, ok := bd.Adjust(oldEntity.ID(), func(entity *unittest.MockEntity) *unittest.MockEntity {
		// oldEntity must be passed to func parameter of adjust.
		require.Equal(t, oldEntityID, entity.ID())
		require.Equal(t, oldEntity, entity)

		// adjust logic, adjsuts the nonce of the entity
		return &unittest.MockEntity{Identifier: oldEntityID, Nonce: 2}
	})

	// adjustment must be successful, and identifier must be updated.
	require.True(t, ok)
	require.Equal(t, oldEntityID, newEntity.ID())
	require.Equal(t, uint64(2), newEntity.Nonce)

	// replaces new entity in the original reference list and
	// retrieves all.
	entities[entityIndex] = newEntity
	testRetrievableFrom(t, bd, entities, 0)

	// Now, re-adjusting the entity (using its original ID) should succeed.
	entity, ok := bd.Adjust(oldEntityID, func(entity *unittest.MockEntity) *unittest.MockEntity {
		// Further adjust: increment the nonce.
		return &unittest.MockEntity{Identifier: oldEntityID, Nonce: entity.Nonce + 1}
	})
	require.True(t, ok)
	require.NotNil(t, entity)
	require.Equal(t, oldEntityID, entity.ID())
	// Check that the nonce was incremented from 2 to 3.
	require.Equal(t, uint64(3), entity.Nonce)

	// Retrieving the entity using the original identifier must succeed.
	entity, ok = bd.Get(oldEntityID)
	require.True(t, ok)
	require.Equal(t, oldEntityID, entity.ID())
	require.Equal(t, uint64(3), entity.Nonce)

	ok = bd.Has(oldEntityID)
	require.True(t, ok)
}

// TestArrayBackData_WriteHeavy evaluates correctness of Cache under the writing and retrieving
// a heavy load of entities up to its limit. All data must be written successfully and then retrievable.
func TestArrayBackData_WriteHeavy(t *testing.T) {
	limit := 100_000

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// retrieves all entities from backdata
	testRetrievableFrom(t, bd, entities, 0)
}

// TestArrayBackData_LRU_Ejection evaluates correctness of Cache under the writing and retrieving
// a heavy load of entities beyond its limit. With LRU ejection, only most recently written data must be maintained
// by mempool.
func TestArrayBackData_LRU_Ejection(t *testing.T) {
	// mempool has the limit of 100K, but we put 1M
	// (10 time more than its capacity)
	limit := 100_000
	items := uint(1_000_000)

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(items)

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// only last 100K (i.e., 900Kth forward) items must be retrievable, and
	// the rest must be ejected.
	testRetrievableFrom(t, bd, entities, 900_000)
}

// TestArrayBackData_No_Ejection evaluates correctness of Cache under the writing and retrieving
// a heavy load of entities beyond its limit. With NoEjection mode, the cache should refuse to add extra entities beyond
// its limit.
func TestArrayBackData_No_Ejection(t *testing.T) {
	// mempool has the limit of 100K, but we put 1M
	// (10 time more than its capacity)
	limit := 100_000
	items := uint(1_000_000)

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.NoEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(items)

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.NoEjection)

	// only last 100K (i.e., 900Kth forward) items must be retrievable, and
	// the rest must be ejected.
	testRetrievableInRange(t, bd, entities, 0, limit)
}

// TestArrayBackData_Random_Ejection evaluates correctness of Cache under the writing and retrieving
// a heavy load of entities beyond its limit. With random ejection, only as many entities as capacity of
// Cache must be retrievable.
func TestArrayBackData_Random_Ejection(t *testing.T) {
	// mempool has the limit of 100K, but we put 1M
	// (10 time more than its capacity)
	limit := 100_000
	items := uint(1_000_000)

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.RandomEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(items)

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.RandomEjection)

	// only 100K (random) items must be retrievable, as the rest
	// are randomly ejected to make room.
	testRetrievableCount(t, bd, entities, 100_000)
}

// TestArrayBackData_AddDuplicate evaluates that adding duplicate entity to Cache will fail without
// altering the internal state of it.
func TestArrayBackData_AddDuplicate(t *testing.T) {
	limit := 100

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// adding duplicate entity should fail
	for _, entity := range entities {
		require.False(t, bd.Add(entity.ID(), entity))
	}

	// still all entities must be retrievable from Cache.
	testRetrievableFrom(t, bd, entities, 0)
}

// TestArrayBackData_Clear evaluates that calling Clear method removes all entities stored in BackData.
func TestArrayBackData_Clear(t *testing.T) {
	limit := 100

	bd := NewCache[*unittest.MockEntity](uint32(limit),
		8,
		heropool.LRUEjection,
		unittest.Logger(),
		metrics.NewNoopCollector())

	entities := unittest.EntityListFixture(uint(limit))

	// adds all entities to backdata
	testAddEntities(t, bd, entities, heropool.LRUEjection)

	// still all must be retrievable from backdata
	testRetrievableFrom(t, bd, entities, 0)
	require.Equal(t, bd.Size(), uint(limit))
	require.Len(t, bd.All(), limit)

	// calling clear must shrink size of BackData to zero
	bd.Clear()
	require.Equal(t, bd.Size(), uint(0))
	require.Len(t, bd.All(), 0)

	// none of stored elements must be retrievable any longer
	testRetrievableCount(t, bd, entities, 0)
}

// TestArrayBackData_All checks correctness of All method in returning all stored entities in it.
func TestArrayBackData_All(t *testing.T) {
	tt := []struct {
		limit        uint32
		items        uint32
		ejectionMode heropool.EjectionMode
	}{
		{ // mempool has the limit of 1000, but we put 100.
			limit:        1000,
			items:        100,
			ejectionMode: heropool.LRUEjection,
		},
		{ // mempool has the limit of 1000, and we put exactly 1000 items.
			limit:        1000,
			items:        1000,
			ejectionMode: heropool.LRUEjection,
		},
		{ // mempool has the limit of 1000, and we put 10K items with LRU ejection.
			limit:        1000,
			items:        10_000,
			ejectionMode: heropool.LRUEjection,
		},
		{ // mempool has the limit of 1000, and we put 10K items with random ejection.
			limit:        1000,
			items:        10_000,
			ejectionMode: heropool.RandomEjection,
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("%d-limit-%d-items-%s-ejection", tc.limit, tc.items, tc.ejectionMode), func(t *testing.T) {
			bd := NewCache[*unittest.MockEntity](tc.limit,
				8,
				tc.ejectionMode,
				unittest.Logger(),
				metrics.NewNoopCollector())
			entities := unittest.EntityListFixture(uint(tc.items))

			testAddEntities(t, bd, entities, tc.ejectionMode)

			if tc.ejectionMode == heropool.RandomEjection {
				// in random ejection mode we count total number of matched entities
				// with All map.
				testMapMatchCount(t, bd.All(), entities, int(tc.limit))
				testEntitiesMatchCount(t, bd.Values(), entities, int(tc.limit))
				testIdentifiersMatchCount(t, bd.Keys(), entities, int(tc.limit))
			} else {
				// in LRU ejection mode we match All items based on a from index (i.e., last "from" items).
				from := int(tc.items) - int(tc.limit)
				if from < 0 {
					// we are below limit, hence we start matching from index 0
					from = 0
				}
				testMapMatchFrom(t, bd.All(), entities, from)
				testEntitiesMatchFrom(t, bd.Values(), entities, from)
				testIdentifiersMatchFrom(t, bd.Keys(), entities, from)
			}
		})
	}
}

// TestArrayBackData_Remove checks correctness of removing elements from Cache.
func TestArrayBackData_Remove(t *testing.T) {
	tt := []struct {
		limit uint32
		items uint32
		from  int // index start to be removed (set -1 to remove randomly)
		count int // total elements to be removed
	}{
		{ // removing range with total items below the limit
			limit: 100_000,
			items: 10_000,
			from:  188,
			count: 2012,
		},
		{ // removing range from full Cache
			limit: 100_000,
			items: 100_000,
			from:  50_333,
			count: 6667,
		},
		{ // removing random from Cache with total items below its limit
			limit: 100_000,
			items: 10_000,
			from:  -1,
			count: 6888,
		},
		{ // removing random from full Cache
			limit: 100_000,
			items: 10_000,
			from:  -1,
			count: 7328,
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("%d-limit-%d-items-%dfrom-%dcount", tc.limit, tc.items, tc.from, tc.count), func(t *testing.T) {
			bd := NewCache[*unittest.MockEntity](
				tc.limit,
				8,
				heropool.RandomEjection,
				unittest.Logger(),
				metrics.NewNoopCollector())
			entities := unittest.EntityListFixture(uint(tc.items))

			testAddEntities(t, bd, entities, heropool.RandomEjection)

			if tc.from == -1 {
				// random removal
				testRemoveAtRandom(t, bd, entities, tc.count)
				// except removed ones, the rest must be retrievable
				testRetrievableCount(t, bd, entities, uint64(int(tc.items)-tc.count))
			} else {
				// removing a range
				testRemoveRange(t, bd, entities, tc.from, tc.from+tc.count)
				testCheckRangeRemoved(t, bd, entities, tc.from, tc.from+tc.count)
			}
		})
	}
}

// testAddEntities is a test helper that checks entities are added successfully to the Cache.
// and each entity is retrievable right after it is written to backdata.
func testAddEntities(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, ejection heropool.EjectionMode) {
	// initially, head should be undefined
	e, ok := bd.Head()
	require.False(t, ok)
	require.Nil(t, e)

	// adding elements
	for i, e := range entities {
		if ejection == heropool.NoEjection && uint32(i) >= bd.sizeLimit {
			// with no ejection when it goes beyond limit, the writes should be unsuccessful.
			require.False(t, bd.Add(e.ID(), e))

			// the head should retrieve the first added entity.
			headEntity, headExists := bd.Head()
			require.True(t, headExists)
			require.Equal(t, headEntity.ID(), entities[0].ID())
		} else {
			// adding each element must be successful.
			require.True(t, bd.Add(e.ID(), e))

			if uint32(i) < bd.sizeLimit {
				// when we are below limit the size of
				// Cache should be incremented by each addition.
				require.Equal(t, bd.Size(), uint(i+1))

				// in case cache is not full, the head should retrieve the first added entity.
				headEntity, headExists := bd.Head()
				require.True(t, headExists)
				require.Equal(t, headEntity.ID(), entities[0].ID())
			} else {
				// when we cross the limit, the ejection kicks in, and
				// size must be steady at the limit.
				require.Equal(t, uint32(bd.Size()), bd.sizeLimit)
			}

			// entity should be immediately retrievable
			actual, ok := bd.Get(e.ID())
			require.True(t, ok)
			require.Equal(t, e, actual)
		}
	}
}

// testRetrievableInRange is a test helper that evaluates that all entities starting from given index are retrievable from Cache.
func testRetrievableFrom(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, from int) {
	testRetrievableInRange(t, bd, entities, from, len(entities))
}

// testRetrievableInRange is a test helper that evaluates within given range [from, to) are retrievable from Cache.
func testRetrievableInRange(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, from int, to int) {
	for i := range entities {
		expected := entities[i]
		actual, ok := bd.Get(expected.ID())
		if i < from || i >= to {
			require.False(t, ok, i)
			require.Nil(t, actual)
		} else {
			require.True(t, ok)
			require.Equal(t, expected, actual)
		}
	}
}

// testRemoveAtRandom is a test helper removes specified number of entities from Cache at random.
func testRemoveAtRandom(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, count int) {
	for removedCount := 0; removedCount < count; {
		unittest.RequireReturnsBefore(t, func() {
			index := rand.Int() % len(entities)
			expected, removed := bd.Remove(entities[index].ID())
			if !removed {
				return
			}
			require.Equal(t, entities[index], expected)
			removedCount++
			// size sanity check after removal
			require.Equal(t, bd.Size(), uint(len(entities)-removedCount))
		}, 100*time.Millisecond, "could not find element to remove")
	}
}

// testRemoveRange is a test helper that removes specified range of entities from Cache.
func testRemoveRange(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, from int, to int) {
	for i := from; i < to; i++ {
		expected, removed := bd.Remove(entities[i].ID())
		require.True(t, removed)
		require.Equal(t, entities[i], expected)
		// size sanity check after removal
		require.Equal(t, bd.Size(), uint(len(entities)-(i-from)-1))
	}
}

// testCheckRangeRemoved is a test helper that evaluates the specified range of entities have been removed from Cache.
func testCheckRangeRemoved(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, from int, to int) {
	for i := from; i < to; i++ {
		// both removal and retrieval must fail
		expected, removed := bd.Remove(entities[i].ID())
		require.False(t, removed)
		require.Nil(t, expected)

		expected, exists := bd.Get(entities[i].ID())
		require.False(t, exists)
		require.Nil(t, expected)
	}
}

// testMapMatchFrom is a test helper that checks entities are retrievable from entitiesMap starting specified index.
func testMapMatchFrom(t *testing.T, entitiesMap map[flow.Identifier]*unittest.MockEntity, entities []*unittest.MockEntity, from int) {
	require.Len(t, entitiesMap, len(entities)-from)

	for i := range entities {
		expected := entities[i]
		actual, ok := entitiesMap[expected.ID()]
		if i < from {
			require.False(t, ok, i)
			require.Nil(t, actual)
		} else {
			require.True(t, ok)
			require.Equal(t, expected, actual)
		}
	}
}

// testEntitiesMatchFrom is a test helper that checks entities are retrievable from given list starting specified index.
func testEntitiesMatchFrom(t *testing.T, expectedEntities []*unittest.MockEntity, actualEntities []*unittest.MockEntity, from int) {
	require.Len(t, expectedEntities, len(actualEntities)-from)

	for i, actual := range actualEntities {
		if i < from {
			require.NotContains(t, expectedEntities, actual)
		} else {
			require.Contains(t, expectedEntities, actual)
		}
	}
}

// testIdentifiersMatchFrom is a test helper that checks identifiers of entities are retrievable from given list starting specified index.
func testIdentifiersMatchFrom(t *testing.T, expectedIdentifiers flow.IdentifierList, actualEntities []*unittest.MockEntity, from int) {
	require.Len(t, expectedIdentifiers, len(actualEntities)-from)

	for i, actual := range actualEntities {
		if i < from {
			require.NotContains(t, expectedIdentifiers, actual.ID())
		} else {
			require.Contains(t, expectedIdentifiers, actual.ID())
		}
	}
}

// testMapMatchFrom is a test helper that checks specified number of entities are retrievable from entitiesMap.
func testMapMatchCount(t *testing.T, entitiesMap map[flow.Identifier]*unittest.MockEntity, entities []*unittest.MockEntity, count int) {
	require.Len(t, entitiesMap, count)
	actualCount := 0
	for i := range entities {
		expected := entities[i]
		actual, ok := entitiesMap[expected.ID()]
		if !ok {
			continue
		}
		require.Equal(t, expected, actual)
		actualCount++
	}
	require.Equal(t, count, actualCount)
}

// testEntitiesMatchCount is a test helper that checks specified number of entities are retrievable from given list.
func testEntitiesMatchCount(t *testing.T, expectedEntities []*unittest.MockEntity, actualEntities []*unittest.MockEntity, count int) {
	entitiesMap := make(map[flow.Identifier]*unittest.MockEntity)

	// converts expected entities list to a map in order to utilize a test helper.
	for _, expected := range expectedEntities {
		entitiesMap[expected.ID()] = expected
	}

	testMapMatchCount(t, entitiesMap, actualEntities, count)
}

// testIdentifiersMatchCount is a test helper that checks specified number of entities are retrievable from given list.
func testIdentifiersMatchCount(t *testing.T, expectedIdentifiers flow.IdentifierList, actualEntities []*unittest.MockEntity, count int) {
	idMap := make(map[flow.Identifier]struct{})

	// converts expected identifiers to a map.
	for _, expectedId := range expectedIdentifiers {
		idMap[expectedId] = struct{}{}
	}

	require.Len(t, idMap, count)
	actualCount := 0
	for _, e := range actualEntities {
		_, ok := idMap[e.ID()]
		if !ok {
			continue
		}
		actualCount++
	}
	require.Equal(t, count, actualCount)
}

// testRetrievableCount is a test helper that checks the number of retrievable entities from backdata exactly matches
// the expectedCount.
func testRetrievableCount(t *testing.T, bd *Cache[*unittest.MockEntity], entities []*unittest.MockEntity, expectedCount uint64) {
	actualCount := 0

	for i := range entities {
		expected := entities[i]
		actual, ok := bd.Get(expected.ID())
		if !ok {
			continue
		}
		require.Equal(t, expected, actual)
		actualCount++
	}

	require.Equal(t, int(expectedCount), actualCount)
}
