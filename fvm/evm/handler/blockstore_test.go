package handler_test

import (
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

	var chainID = flow.Testnet
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			bs := handler.NewBlockStore(chainID, backend, root)

			// check the Genesis block
			b, err := bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlock(chainID), b)
			h, err := bs.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash(chainID), h)

			// test block proposal construction from the Genesis block
			bp, err := bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(1), bp.Height)
			expectedParentHash, err := types.GenesisBlock(chainID).Hash()
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
			bs = handler.NewBlockStore(chainID, backend, root)
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
			require.Equal(t, types.GenesisBlock(chainID), retb)

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
			require.Equal(t, types.GenesisBlockHash(chainID), h)

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
