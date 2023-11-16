package handler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestBlockStore(t *testing.T) {

	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			bs, err := handler.NewBlockStore(backend, root)
			require.NoError(t, err)

			// check gensis block
			b, err := bs.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlock, b)

			// test block proposal from genesis
			bp, err := bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(1), bp.Height)
			expectedParentHash, err := types.GenesisBlock.Hash()
			require.NoError(t, err)
			require.Equal(t, expectedParentHash, bp.ParentBlockHash)

			// commit block proposal
			supply := uint64(100)
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
		})

	})

}
