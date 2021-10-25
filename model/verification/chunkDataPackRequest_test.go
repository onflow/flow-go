package verification_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkDataRequestList_Contain(t *testing.T) {
	list := unittest.ChunkDataPackRequestListFixture(10)

	t.Run("contains returns true for all existing elements", func(t *testing.T) {
		for _, req := range list {
			require.True(t, list.Contains(&verification.ChunkDataPackRequest{
				Locator: chunks.Locator{
					ResultID: req.ResultID,
					Index:    req.Index,
				},
			}))
		}
	})

	t.Run("contains returns false for non-existing element", func(t *testing.T) {
		require.False(t, list.Contains(&verification.ChunkDataPackRequest{
			Locator: chunks.Locator{
				ResultID: unittest.IdentifierFixture(),
				Index:    uint64(10),
			},
		}))
	})
}
