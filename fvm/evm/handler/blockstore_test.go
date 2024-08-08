package handler_test

import (
	"math/big"
	"testing"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethRLP "github.com/onflow/go-ethereum/rlp"
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

			// check the Genesis block
			b, err := bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlock, b)
			h, err := bs.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash, h)

			// test block proposal construction from the Genesis block
			bp, err := bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(1), bp.Height)
			expectedParentHash, err := types.GenesisBlock.Hash()
			require.NoError(t, err)
			require.Equal(t, expectedParentHash, bp.ParentBlockHash)

			// if no commit and again block proposal call should return the same
			retbp, err := bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, bp, retbp)

			// update the block proposal
			bp.TotalGasUsed += 100
			err = bs.UpdateBlockProposal(bp)
			require.NoError(t, err)

			// reset the bs and check if it still return the block proposal
			bs = handler.NewBlockStore(backend, root)
			retbp, err = bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, bp, retbp)

			// update the block proposal again
			supply := big.NewInt(100)
			bp.TotalSupply = supply
			err = bs.UpdateBlockProposal(bp)
			require.NoError(t, err)
			// this should still return the genesis block
			retb, err := bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlock, retb)

			// commit the changes
			err = bs.CommitBlockProposal(bp)
			require.NoError(t, err)
			retb, err = bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, supply, retb.TotalSupply)
			require.Equal(t, uint64(1), retb.Height)

			retbp, err = bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(2), retbp.Height)

			// check block hashes
			// genesis
			h, err = bs.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash, h)

			// block 1
			h, err = bs.BlockHash(1)
			require.NoError(t, err)
			expected, err := bp.Hash()
			require.NoError(t, err)
			require.Equal(t, expected, h)

			// block 2
			h, err = bs.BlockHash(2)
			require.NoError(t, err)
			require.Equal(t, gethCommon.Hash{}, h)
		})

	})

}

// TODO: we can remove this when the previewnet is out
func TestBlockStoreMigration(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			legacyCapacity := 16
			maxHeightAdded := 32
			legacy := types.NewBlockHashList(16)
			for i := 0; i <= maxHeightAdded; i++ {
				err := legacy.Push(uint64(i), gethCommon.Hash{byte(i)})
				require.NoError(t, err)
			}
			err := backend.SetValue(
				root[:],
				[]byte(handler.BlockStoreBlockHashesKey),
				legacy.Encode(),
			)
			require.NoError(t, err)
			bs := handler.NewBlockStore(backend, root)

			for i := 0; i <= maxHeightAdded-legacyCapacity; i++ {
				h, err := bs.BlockHash(uint64(i))
				require.NoError(t, err)
				require.Equal(t, gethCommon.Hash{}, h)
			}

			for i := maxHeightAdded - legacyCapacity + 1; i <= maxHeightAdded; i++ {
				h, err := bs.BlockHash(uint64(i))
				require.NoError(t, err)
				require.Equal(t, gethCommon.Hash{byte(i)}, h)
			}
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

			err = bs.ResetBlockProposal()
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

			blockBytes3, err := gethRLP.EncodeToBytes(bp.Block)
			require.NoError(t, err)

			err = backend.SetValue(root[:], []byte(handler.BlockStoreLatestBlockKey), blockBytes3)
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
