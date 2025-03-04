package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMapBackData_StoreAnd(t *testing.T) {
	backData := NewMapBackData[flow.Identifier, *unittest.MockEntity]()
	entities := unittest.EntityListFixture(100)

	// Add
	for _, e := range entities {
		// all values must be stored successfully
		require.True(t, backData.Add(e.ID(), e))
	}

	// Get
	for _, expected := range entities {
		// all values must be retrievable successfully
		actual, ok := backData.Get(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.Get(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// Keys
	ids := backData.Keys()
	require.Equal(t, len(entities), len(ids))
	for _, id := range ids {
		require.True(t, backData.Has(id))
	}

	// Values
	actualValues := backData.Values()
	require.Equal(t, len(entities), len(actualValues))
	require.ElementsMatch(t, entities, actualValues)
}

// TestMapBackData_AdjustWithInit tests the AdjustWithInit method of the MapBackData.
// Note that as the backdata is not inherently thread-safe, this test is not concurrent.
func TestMapBackData_AdjustWithInit(t *testing.T) {
	backData := NewMapBackData[flow.Identifier, *unittest.MockEntity]()
	entities := unittest.EntityListFixture(100)
	ids := flow.GetIDs(entities)

	// AdjustWithInit
	for _, e := range entities {
		// all values must be adjusted successfully
		actual, ok := backData.AdjustWithInit(e.ID(), func(entity *unittest.MockEntity) *unittest.MockEntity {
			// increment nonce of the entity
			entity.Nonce++
			return entity
		}, func() *unittest.MockEntity {
			return e
		})
		require.True(t, ok)
		require.Equal(t, e, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.Get(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.Nonce)
	}

	// Keys
	retriedIds := backData.Keys()
	require.Equal(t, len(entities), len(retriedIds))
	require.ElementsMatch(t, ids, retriedIds)
	for _, key := range retriedIds {
		require.True(t, backData.Has(key))
	}

	// Values
	actualValues := backData.Values()
	require.Equal(t, len(entities), len(actualValues))
	require.ElementsMatch(t, entities, actualValues)

	// Get
	for _, e := range entities {
		// all values must be retrieved successfully
		actual, ok := backData.Get(e.ID())
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.Nonce)
	}

	// GetWithInit
	for _, e := range entities {
		// all values must be retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() *unittest.MockEntity {
			require.Fail(t, "should not be called") // value has already been initialized
			return e
		})
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.Nonce)
	}
}

// TestMapBackData_GetWithInit tests the GetWithInit method of the MapBackData.
// Note that as the backdata is not inherently thread-safe, this test is not concurrent.
func TestMapBackData_GetWithInit(t *testing.T) {
	backData := NewMapBackData[flow.Identifier, *unittest.MockEntity]()
	entities := unittest.EntityListFixture(100)

	// GetWithInit
	for _, e := range entities {
		// all values must be initialized retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() *unittest.MockEntity {
			return e // initialize with the value
		})
		require.True(t, ok)
		require.Equal(t, e, actual)
	}

	// All
	all := backData.All()
	require.Equal(t, len(entities), len(all))
	for _, expected := range entities {
		actual, ok := backData.Get(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	// Keys
	ids := backData.Keys()
	require.Equal(t, len(entities), len(ids))
	for _, id := range ids {
		require.True(t, backData.Has(id))
	}

	// Values
	actualKeys := backData.Values()
	require.Equal(t, len(entities), len(actualKeys))
	require.ElementsMatch(t, entities, actualKeys)

	// Adjust
	for _, v := range entities {
		// all values must be adjusted successfully
		actual, ok := backData.Adjust(v.ID(), func(entity *unittest.MockEntity) *unittest.MockEntity {
			// increment nonce of the entity
			entity.Nonce++
			return entity
		})
		require.True(t, ok)
		require.Equal(t, v, actual)
	}

	// Get; should return the latest version of the entity
	for _, e := range entities {
		// all values must be retrieved successfully
		actual, ok := backData.Get(e.ID())
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.Nonce)
	}

	// GetWithInit; should return the latest version of the entity, than increment the nonce
	for _, e := range entities {
		// all values must be retrieved successfully
		actual, ok := backData.GetWithInit(e.ID(), func() *unittest.MockEntity {
			require.Fail(t, "should not be called") // entity has already been initialized
			return e
		})
		require.True(t, ok)
		require.Equal(t, e.ID(), actual.ID())
		require.Equal(t, uint64(1), actual.Nonce)
	}
}
