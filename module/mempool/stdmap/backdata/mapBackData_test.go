package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMapBackData_StoreAnd(t *testing.T) {
	backData := NewMapBackData()
	entities := unittest.EntityListFixture(100)

	// Add
	for _, e := range entities {
		// all entities must be stored successfully
		require.True(t, backData.Add(e.ID(), e))
	}

	// ByID
	for _, expected := range entities {
		// all entities must be retrievable successfully
		actual, ok := backData.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// Identifiers
	ids := backData.Identifiers()
	require.Equal(t, len(entities), len(ids))
	for _, id := range ids {
		require.True(t, backData.Has(id))
	}

	// Entities
	actualEntities := backData.Entities()
	require.Equal(t, len(entities), len(actualEntities))
	require.ElementsMatch(t, entities, actualEntities)
}

// TestMapBackData_AdjustWithInit tests the AdjustWithInit method of the MapBackData.
// Note that as the backdata is not inherently thread-safe, this test is not concurrent.
func TestMapBackData_AdjustWithInit(t *testing.T) {
	backData := NewMapBackData()
	entities := unittest.EntityListFixture(100)
	ids := flow.GetIDs(entities)

	// AdjustWithInit
	for _, e := range entities {
		// all entities must be adjusted successfully
		actual, ok := backData.AdjustWithInit(e.ID(), func(entity flow.Entity) flow.Entity {
			// increment nonce of the entity
			entity.(*unittest.MockEntity).Nonce++
			return entity
		}, func() flow.Entity {
			return e
		})
		require.True(t, ok)
		require.Equal(t, e, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.(*unittest.MockEntity).Nonce)
	}

	// Identifiers
	retriedIds := backData.Identifiers()
	require.Equal(t, len(entities), len(retriedIds))
	require.ElementsMatch(t, ids, retriedIds)
	for _, id := range retriedIds {
		require.True(t, backData.Has(id))
	}

	// Entities
	actualEntities := backData.Entities()
	require.Equal(t, len(entities), len(actualEntities))
	require.ElementsMatch(t, entities, actualEntities)

	// ByID
	for _, e := range entities {
		// all entities must be retrieved successfully
		actual, ok := backData.ByID(e.ID())
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.(*unittest.MockEntity).Nonce)
	}

	// GetWithInit
	for _, e := range entities {
		// all entities must be retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() flow.Entity {
			require.Fail(t, "should not be called") // entity has already been initialized
			return e
		})
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.(*unittest.MockEntity).Nonce)
	}
}

// TestMapBackData_GetWithInit tests the GetWithInit method of the MapBackData.
// Note that as the backdata is not inherently thread-safe, this test is not concurrent.
func TestMapBackData_GetWithInit(t *testing.T) {
	backData := NewMapBackData()
	entities := unittest.EntityListFixture(100)

	// GetWithInit
	for _, e := range entities {
		// all entities must be initialized retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() flow.Entity {
			return e // initialize with the entity
		})
		require.True(t, ok)
		require.Equal(t, e, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// Identifiers
	ids := backData.Identifiers()
	require.Equal(t, len(entities), len(ids))
	for _, id := range ids {
		require.True(t, backData.Has(id))
	}

	// Entities
	actualEntities := backData.Entities()
	require.Equal(t, len(entities), len(actualEntities))
	require.ElementsMatch(t, entities, actualEntities)

	// Adjust
	for _, e := range entities {
		// all entities must be adjusted successfully
		actual, ok := backData.Adjust(e.ID(), func(entity flow.Entity) flow.Entity {
			// increment nonce of the entity
			entity.(*unittest.MockEntity).Nonce++
			return entity
		})
		require.True(t, ok)
		require.Equal(t, e, actual)
	}

	// ByID; should return the latest version of the entity
	for _, e := range entities {
		// all entities must be retrieved successfully
		actual, ok := backData.ByID(e.ID())
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.(*unittest.MockEntity).Nonce)
	}

	// GetWithInit; should return the latest version of the entity, than increment the nonce
	for _, e := range entities {
		// all entities must be retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() flow.Entity {
			require.Fail(t, "should not be called") // entity has already been initialized
			return e
		})
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.(*unittest.MockEntity).Nonce)
	}
}
