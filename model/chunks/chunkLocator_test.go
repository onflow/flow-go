package chunks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkLocatorList_Contains(t *testing.T) {
	list := unittest.ChunkLocatorListFixture(10)

	t.Run("contains returns true for all existing elements", func(t *testing.T) {
		for _, l := range list {
			require.True(t, list.Contains(&chunks.Locator{
				ResultID: l.ResultID,
				Index:    l.Index,
			}))
		}
	})

	t.Run("contains returns false for non-existing element", func(t *testing.T) {
		require.False(t, list.Contains(&chunks.Locator{
			ResultID: unittest.IdentifierFixture(),
			Index:    uint64(10),
		}))
	})
}
