package handler_test

import (
	gethRLP "github.com/onflow/go-ethereum/rlp"
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlockStore(t *testing.T) {

	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			bs := handler.NewBlockStore(backend, root)

			// check gensis block
			b, err := bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlock, b)
			h, err := bs.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash, h)

			// test block proposal from genesis
			bp, err := bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(1), bp.Height)
			expectedParentHash, err := types.GenesisBlock.Hash()
			require.NoError(t, err)
			require.Equal(t, expectedParentHash, bp.ParentBlockHash)

			// commit block proposal
			supply := big.NewInt(100)
			bp.TotalSupply = supply
			err = bs.CommitBlockProposal()
			require.NoError(t, err)
			b, err = bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, supply, b.TotalSupply)
			require.Equal(t, uint64(1), b.Height)
			bp, err = bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(2), bp.Height)

			// check block hashes
			// genesis
			h, err = bs.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash, h)

			// block 1
			h, err = bs.BlockHash(1)
			require.NoError(t, err)
			expected, err := b.Hash()
			require.NoError(t, err)
			require.Equal(t, expected, h)

			// block 2
			h, err = bs.BlockHash(2)
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{}, h)
		})

	})

}

// This test reproduces a state before a breaking change on the Block type,
// which added a timestamp and total gas used,
// then it adds new blocks and makes sure the retrival
// and storage of blocks works as it should, the breaking change was introduced
// in this PR https://github.com/onflow/flow-go/pull/5660
func TestBlockStore_AddedTimestamp(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {

			bs := handler.NewBlockStore(backend, root)

			// block type before breaking change (no timestamp and total gas used)
			type oldBlockV1 struct {
				ParentBlockHash   gethCommon.Hash
				Height            uint64
				UUIDIndex         uint64
				TotalSupply       uint64
				StateRoot         gethCommon.Hash
				ReceiptRoot       gethCommon.Hash
				TransactionHashes []gethCommon.Hash
			}

			g := types.GenesisBlock
			h, err := g.Hash()
			require.NoError(t, err)

			b := oldBlockV1{
				ParentBlockHash:   h,
				Height:            1,
				ReceiptRoot:       g.ReceiptRoot,
				UUIDIndex:         123,
				TotalSupply:       1,
				StateRoot:         h,
				TransactionHashes: []gethCommon.Hash{h},
			}
			blockBytes, err := gethRLP.EncodeToBytes(b)
			require.NoError(t, err)

			// store a block without timestamp, simulate existing state before the breaking change
			err = backend.SetValue(root[:], []byte(handler.BlockStoreLatestBlockKey), blockBytes)
			require.NoError(t, err)

			block, err := bs.LatestBlock()
			require.NoError(t, err)

			require.Empty(t, block.Timestamp)
			require.Empty(t, block.TotalGasUsed)
			require.Equal(t, b.Height, block.Height)
			require.Equal(t, b.ParentBlockHash, block.ParentBlockHash)
			require.Equal(t, b.ReceiptRoot, block.ReceiptRoot)

			// added timestamp
			type oldBlockV2 struct {
				ParentBlockHash   gethCommon.Hash
				Height            uint64
				Timestamp         uint64
				TotalSupply       *big.Int
				ReceiptRoot       gethCommon.Hash
				TransactionHashes []gethCommon.Hash
			}

			b2 := oldBlockV2{
				ParentBlockHash: h,
				Height:          2,
				TotalSupply:     g.TotalSupply,
				ReceiptRoot:     g.ReceiptRoot,
				Timestamp:       1,
			}
			blockBytes2, err := gethRLP.EncodeToBytes(b2)
			require.NoError(t, err)

			// store a block without timestamp, simulate existing state before the breaking change
			err = backend.SetValue(root[:], []byte(handler.BlockStoreLatestBlockKey), blockBytes2)
			require.NoError(t, err)

			block2, err := bs.LatestBlock()
			require.NoError(t, err)

			require.Empty(t, block2.TotalGasUsed)
			require.Equal(t, b2.Height, block2.Height)
			require.Equal(t, b2.ParentBlockHash, block2.ParentBlockHash)
			require.Equal(t, b2.TotalSupply, block2.TotalSupply)
			require.Equal(t, b2.ReceiptRoot, block2.ReceiptRoot)
			require.Equal(t, b2.Timestamp, block2.Timestamp)

			bp, err := bs.BlockProposal()
			require.NoError(t, err)

			blockBytes, err = bp.ToBytes()
			require.NoError(t, err)

			err = backend.SetValue(root[:], []byte(handler.BlockStoreLatestBlockKey), blockBytes)
			require.NoError(t, err)

			bb, err := bs.LatestBlock()
			require.NoError(t, err)
			require.NotNil(t, bb.ParentBlockHash)
			require.NotNil(t, bb.TotalGasUsed)
			require.NotNil(t, bb.Timestamp)
			require.Equal(t, b2.Height+1, bb.Height)
		})
	})
}
