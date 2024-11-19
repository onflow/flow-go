package handler_test

import (
	"fmt"
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

			// fmt.Println(backend.Dump())
			pair, _ := backend.Dump()
			for k, v := range pair {
				fmt.Println(k, v)
			}

			// first add blocks for the full range of capacity
			for i := 0; i < capacity; i++ {
				err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
				require.NoError(t, err)
				require.Equal(t, uint64(0), bhl.MinAvailableHeight())
				require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
				h, err := bhl.LastAddedBlockHash()
				require.NoError(t, err)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)

				// check the value for all of them
				for h := 0; h <= i; h++ {
					found, bh, err := bhl.BlockHashByHeight(uint64(h))
					require.NoError(t, err)
					require.True(t, found)
					require.Equal(t, gethCommon.Hash{byte(h)}, bh)
				}
			}

			additional := capacity * 2

			// over the border additions
			for i := capacity; i < capacity+additional; i++ {
				err := bhl.Push(uint64(i), gethCommon.Hash{byte(i)})
				require.NoError(t, err)
				require.Equal(t, uint64(i-capacity+1), bhl.MinAvailableHeight())
				require.Equal(t, uint64(i), bhl.MaxAvailableHeight())

				// check that old block has been replaced
				for h := 0; h < i; h++ {
					expectedFound := h > i-capacity
					found, bh, err := bhl.BlockHashByHeight(uint64(h))
					require.NoError(t, err)
					require.Equal(t, expectedFound, found, fmt.Sprintf("i %v, h: %v", i, h))

					if expectedFound {
						require.Equal(t, gethCommon.Hash{byte(h)}, bh)
					}
				}
			}

			h, err = bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{byte(capacity + additional - 1)}, h)

			// construct a new one and check
			bhl, err = handler.NewBlockHashList(backend, root, capacity)
			require.NoError(t, err)
			require.False(t, bhl.IsEmpty())

			h2, err := bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, h, h2)

			require.Equal(t, uint64(additional), bhl.MinAvailableHeight())
			require.Equal(t, uint64(capacity+additional-1), bhl.MaxAvailableHeight())

			// check all the stored blocks
			for i := additional; i < capacity+additional; i++ {
				found, h, err := bhl.BlockHashByHeight(uint64(i))
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}
		})
	})
}

func TestList(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			capacity := 4
			bhl, err := handler.NewBlockHashList(backend, root, capacity)
			require.NoError(t, err)
			require.True(t, bhl.IsEmpty())

			h, err := bhl.LastAddedBlockHash()
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{}, h)

			for i := 0; i < 20; i++ {
				err := bhl.Push(uint64(i), gethCommon.Hash{byte(i + 1)})
				require.NoError(t, err)
				// require.Equal(t, uint64(0), bhl.MinAvailableHeight())
				// require.Equal(t, uint64(i), bhl.MaxAvailableHeight())
				h, err := bhl.LastAddedBlockHash()
				require.NoError(t, err)
				require.Equal(t, gethCommon.Hash{byte(i + 1)}, h)

				// data, _ := backend.Dump()
				// for k, v := range data {
				// 	fmt.Printf("%v: %x\n", k, v)
				// }
			}

			found, h, err := bhl.BlockHashByHeight(19)
			require.NoError(t, err)
			require.True(t, found)
		})
	})
}
