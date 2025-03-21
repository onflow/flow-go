package chunks_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkLocatorConvert evaluates converting chunk locator list to map and vice versa.
func TestChunkLocatorConvert(t *testing.T) {
	originalList := unittest.ChunkLocatorListFixture(10)
	locatorMap := originalList.ToMap()

	require.Equal(t, len(originalList), len(locatorMap))
	for _, locator := range originalList {
		_, ok := locatorMap[locator.ID()]
		require.True(t, ok, "missing chunk locator in conversion from list to map")
	}

	convertedList := locatorMap.ToList()
	require.ElementsMatch(t, originalList, convertedList)
}

// TestChunkLocatorMalleability verifies that the chunk locator which implements the [flow.IDEntity] interface is not malleable.
func TestChunkLocatorMalleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), rand.Uint64()))
}
