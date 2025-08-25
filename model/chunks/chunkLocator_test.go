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

// TestLocator_EqualTo verifies the correctness of the EqualTo method on Locator.
// It checks that Locators are considered equal if and only if all fields match.
func TestLocator_EqualTo(t *testing.T) {
	loc1 := unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 0)
	loc2 := unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 1)

	require.False(t, loc1.EqualTo(loc2), "Initially, all fields differ; EqualTo should return false")

	// List of mutations to gradually make loc1 equal to loc2
	mutations := []func(){
		func() {
			loc1.ResultID = loc2.ResultID
		},
		func() {
			loc1.Index = loc2.Index
		},
	}

	// Shuffle mutation order
	rand.Shuffle(len(mutations), func(i, j int) {
		mutations[i], mutations[j] = mutations[j], mutations[i]
	})

	// Apply each mutation one at a time, except the last
	for _, mutation := range mutations[:len(mutations)-1] {
		mutation()
		require.False(t, loc1.EqualTo(loc2))
	}

	// Final mutation: should now be equal
	mutations[len(mutations)-1]()
	require.True(t, loc1.EqualTo(loc2))
}

// TestLocator_EqualTo_Nil verifies the behavior of EqualTo when one or both inputs are nil.
func TestLocator_EqualTo_Nil(t *testing.T) {
	var nilLoc *chunks.Locator
	nonNil := unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 0)

	t.Run("nil receiver", func(t *testing.T) {
		require.False(t, nilLoc.EqualTo(nonNil))
	})

	t.Run("nil input", func(t *testing.T) {
		require.False(t, nonNil.EqualTo(nilLoc))
	})

	t.Run("both nil", func(t *testing.T) {
		require.True(t, nilLoc.EqualTo(nil))
	})
}
