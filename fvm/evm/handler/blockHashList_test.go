package handler_test

import (
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlockHashList(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			capacity := 256
			bhl, err := handler.NewBlockHashList(backend, root, capacity)
			require.NoError(t, err)
			require.True(t, bhl.IsEmpty())

			h, err := bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{}, h)

			found, h, err := bhl.BlockHashByHeight(0)
			require.False(t, found)
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{}, h)

			// first add blocks for the full range of capacity
			for i := 0; i < capacity; i++ {
				err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
				require.NoError(t, err)
				require.Equal(t, uint64(0), bhl.MinAvailableHeight())
				require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
				h, err := bhl.LastAddedBlockHash()
				require.NoError(t, err)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}

			// check the value for all of them
			for i := 0; i < capacity; i++ {
				found, h, err := bhl.BlockHashByHeight(uint64(i))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}
			h, err = bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{byte(capacity - 1)}, h)

			// over the border additions
			for i := capacity; i < capacity+3; i++ {
				err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
				require.NoError(t, err)
				require.Equal(t, uint64(i-capacity+1), bhl.MinAvailableHeight())
				require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
			}
			// check that old block has been replaced
			for i := 0; i < 3; i++ {
				found, _, err := bhl.BlockHashByHeight(uint64(i))
				require.NoError(t, err)
				require.False(t, found)
			}
			// check the rest of blocks
			for i := 3; i < capacity+3; i++ {
				found, h, err := bhl.BlockHashByHeight(uint64(i))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}
			h, err = bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{byte(capacity + 2)}, h)

			// construct a new one and check
			bhl, err = handler.NewBlockHashList(backend, root, capacity)
			require.NoError(t, err)
			require.False(t, bhl.IsEmpty())

			h2, err := bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, h, h2)

			require.Equal(t, uint64(3), bhl.MinAvailableHeight())
			require.Equal(t, uint64(capacity+2), bhl.MaxAvailableHeight())

			// check all the stored blocks
			for i := 3; i < capacity+3; i++ {
				found, h, err := bhl.BlockHashByHeight(uint64(i))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}
		})
	})
}
