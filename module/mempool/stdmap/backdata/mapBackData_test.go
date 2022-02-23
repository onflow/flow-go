package backdata

import (
	"testing"

	"github.com/stretchr/testify/require"

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
