package chunks_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
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

// TestNewLocator tests the NewLocator constructor with valid and invalid inputs.
//
// Valid Case:
//
// 1. Valid input with non-zero ResultID and any index:
//   - Should successfully construct a Locator.
//
// Invalid Case:
//
// 2. Invalid input with zero ResultID:
//   - Should return an error indicating ResultID must not be zero.
func TestNewLocator(t *testing.T) {
	t.Run("valid input with non-zero ResultID", func(t *testing.T) {
		locator, err := chunks.NewLocator(
			chunks.UntrustedLocator{
				ResultID: unittest.IdentifierFixture(),
				Index:    1,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, locator)
	})

	t.Run("invalid input with zero ResultID", func(t *testing.T) {
		locator, err := chunks.NewLocator(
			chunks.UntrustedLocator{
				ResultID: flow.ZeroID,
				Index:    1,
			},
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ResultID must not be zero")
		require.Nil(t, locator)
	})
}
