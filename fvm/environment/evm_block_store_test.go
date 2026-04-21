package environment_test

import (
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// TestBlockStoreLifecycle tests the block store lifecycle across two Flow blocks,
// following the execution flow documented in BlockStore comments.
//
// This test verifies:
// - Each Cadence tx creates a new BlockStore instance with empty cache
// - Cache hits/misses work correctly within a Cadence tx
// - FlushBlockProposal persists the proposal for the next Cadence tx
// - Reset discards changes on failed Cadence tx
// - CommitBlockProposal finalizes the block and clears LatestBlockProposal
// - Lazy construction of new proposal in the next Flow block
func TestBlockStoreLifecycle(t *testing.T) {
	chainID := flow.Testnet
	testutils.RunWithTestBackend(t, chainID, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {

			// Gas constants for each EVM tx - using distinct digit positions for easy verification
			// Each tx uses a different decimal place, so accumulated values are easy to read
			const (
				evmTxAGas = uint64(1)      // ones place
				evmTxBGas = uint64(20)     // tens place
				evmTxCGas = uint64(300)    // hundreds place
				evmTxDGas = uint64(4000)   // thousands place
				evmTxEGas = uint64(50000)  // ten-thousands place
				evmTxFGas = uint64(600000) // hundred-thousands place
			)

			// TotalSupply changes for each EVM tx - also using distinct digit positions
			// TotalSupply represents native token deposited in EVM (can increase via deposits)
			var (
				evmTxASupply = big.NewInt(100)      // ones place (x100)
				evmTxBSupply = big.NewInt(2000)     // tens place (x100)
				evmTxESupply = big.NewInt(5000000)  // ten-thousands place (x100)
				evmTxFSupply = big.NewInt(60000000) // hundred-thousands place (x100)
			)

			// ============================================
			// Flow Block K
			// ============================================

			// --- Cadence tx 1 (succeed) ---
			// Each Cadence tx creates a new BlockStore instance
			bs1 := environment.NewBlockStore(chainID, backend, backend, backend, root)

			// EVM Tx A: BlockProposal() - cache miss, loads from storage
			// Since this is the first ever call, LatestBlockProposal is empty,
			// so it reads LatestBlock (genesis) to construct proposal
			bpA, err := bs1.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(1), bpA.Height)
			expectedParentHash, err := types.GenesisBlock(chainID).Hash()
			require.NoError(t, err)
			require.Equal(t, expectedParentHash, bpA.ParentBlockHash)
			require.Equal(t, uint64(0), bpA.TotalGasUsed) // starts at 0

			// Verify TotalSupply is inherited from genesis block (0)
			require.Equal(t, big.NewInt(0), bpA.TotalSupply)

			// Verify PrevRandao is set (non-zero) - read from RandomGenerator during construction
			require.NotEqual(t, gethCommon.Hash{}, bpA.PrevRandao)
			prevRandaoBlockK := bpA.PrevRandao // save for later comparison

			// EVM Tx A: StageBlockProposal() - update cache (accumulate gas and supply)
			bpA.TotalGasUsed += evmTxAGas
			bpA.TotalSupply = new(big.Int).Add(bpA.TotalSupply, evmTxASupply)
			bs1.StageBlockProposal(bpA)

			// EVM Tx B: BlockProposal() - cache hit (same BlockStore instance)
			bpB, err := bs1.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, bpA, bpB)                         // same pointer, cache hit
			require.Equal(t, uint64(1), bpB.TotalGasUsed)      // sees Tx A's gas
			require.Equal(t, big.NewInt(100), bpB.TotalSupply) // sees Tx A's supply
			require.Equal(t, prevRandaoBlockK, bpB.PrevRandao) // PrevRandao unchanged within block

			// EVM Tx B: StageBlockProposal() - update cache (accumulate gas and supply)
			bpB.TotalGasUsed += evmTxBGas
			bpB.TotalSupply = new(big.Int).Add(bpB.TotalSupply, evmTxBSupply)
			bs1.StageBlockProposal(bpB)

			// [tx end]: FlushBlockProposal() - write LatestBlockProposal to storage
			// At this point, TotalGasUsed = 1 + 20 = 21, TotalSupply = 100 + 2000 = 2100
			require.Equal(t, uint64(21), bpB.TotalGasUsed)
			require.Equal(t, big.NewInt(2100), bpB.TotalSupply)
			err = bs1.FlushBlockProposal()
			require.NoError(t, err)

			// --- Cadence tx 2 (failed) ---
			// New BlockStore instance (simulating new Cadence tx)
			bs2 := environment.NewBlockStore(chainID, backend, backend, backend, root)

			// EVM Tx C: BlockProposal() - cache miss, loads from storage
			// Should load the flushed proposal from tx 1 with accumulated gas = 21, supply = 2100
			bpC, err := bs2.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(21), bpC.TotalGasUsed)      // persisted from tx 1
			require.Equal(t, big.NewInt(2100), bpC.TotalSupply) // persisted from tx 1
			require.Equal(t, prevRandaoBlockK, bpC.PrevRandao)  // PrevRandao unchanged within block

			// EVM Tx C: StageBlockProposal() - update cache (accumulate gas)
			bpC.TotalGasUsed += evmTxCGas
			bs2.StageBlockProposal(bpC)

			// EVM Tx D: BlockProposal() - cache hit
			bpD, err := bs2.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(321), bpD.TotalGasUsed) // sees A+B+C = 1+20+300

			// EVM Tx D: StageBlockProposal() - update cache (accumulate gas)
			bpD.TotalGasUsed += evmTxDGas
			bs2.StageBlockProposal(bpD)

			// Verify all 4 txs' gas is accumulated before revert = 1+20+300+4000 = 4321
			require.Equal(t, uint64(4321), bpD.TotalGasUsed)

			// [tx fail/revert]: Reset() - cache = nil, storage unchanged
			bs2.ResetBlockProposal()

			// Verify storage is unchanged by creating a new BlockStore and reading
			// Should only see gas from Cadence tx 1 (A+B = 21), not from failed tx 2 (C+D)
			bs2Check := environment.NewBlockStore(chainID, backend, backend, backend, root)
			bpCheck, err := bs2Check.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(21), bpCheck.TotalGasUsed)      // unchanged from tx 1
			require.Equal(t, big.NewInt(2100), bpCheck.TotalSupply) // unchanged from tx 1

			// --- System chunk tx (last) ---
			// New BlockStore instance for system tx
			bsSystem := environment.NewBlockStore(chainID, backend, backend, backend, root)

			// heartbeat() -> CommitBlockProposal()
			bpCommit, err := bsSystem.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(21), bpCommit.TotalGasUsed)      // A+B = 21
			require.Equal(t, big.NewInt(2100), bpCommit.TotalSupply) // A+B supply = 2100

			// CommitBlockProposal: write LatestBlock, remove LatestBlockProposal
			err = bsSystem.CommitBlockProposal(bpCommit)
			require.NoError(t, err)

			// Verify LatestBlock is now the committed block with accumulated values
			latestBlock, err := bsSystem.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, uint64(1), latestBlock.Height)
			require.Equal(t, uint64(21), latestBlock.TotalGasUsed)
			require.Equal(t, big.NewInt(2100), latestBlock.TotalSupply)
			require.Equal(t, prevRandaoBlockK, latestBlock.PrevRandao) // PrevRandao preserved in block

			// ============================================
			// Flow Block K+1
			// ============================================

			// --- Cadence tx 1 ---
			// New BlockStore instance (new Flow block, new Cadence tx)
			bsK1 := environment.NewBlockStore(chainID, backend, backend, backend, root)

			// EVM Tx E: BlockProposal() - cache miss
			// After CommitBlockProposal, LatestBlockProposal was cleared (or new one written)
			// This tests lazy construction: reads LatestBlock to construct new proposal
			bpE, err := bsK1.BlockProposal()
			require.NoError(t, err)

			// Verify the new proposal is a child of the committed block
			require.Equal(t, uint64(2), bpE.Height) // height incremented
			committedBlockHash, err := latestBlock.Hash()
			require.NoError(t, err)
			require.Equal(t, committedBlockHash, bpE.ParentBlockHash) // parent is committed block

			// Verify the proposal starts fresh (no accumulated gas from previous block)
			require.Equal(t, uint64(0), bpE.TotalGasUsed)

			// Verify TotalSupply is inherited from the committed block (2100)
			require.Equal(t, big.NewInt(2100), bpE.TotalSupply)

			// Verify PrevRandao is set and DIFFERENT from block K (new random value for new block)
			require.NotEqual(t, gethCommon.Hash{}, bpE.PrevRandao)
			require.NotEqual(t, prevRandaoBlockK, bpE.PrevRandao) // different random for new block
			prevRandaoBlockK1 := bpE.PrevRandao

			// EVM Tx E: StageBlockProposal() - update cache (accumulate gas and supply)
			bpE.TotalGasUsed += evmTxEGas
			bpE.TotalSupply = new(big.Int).Add(bpE.TotalSupply, evmTxESupply)
			bsK1.StageBlockProposal(bpE)

			// EVM Tx F: BlockProposal() - cache hit
			bpF, err := bsK1.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(50000), bpF.TotalGasUsed)      // sees Tx E's gas
			require.Equal(t, big.NewInt(5002100), bpF.TotalSupply) // 2100 + 5000000
			require.Equal(t, prevRandaoBlockK1, bpF.PrevRandao)    // PrevRandao unchanged within block

			// EVM Tx F: StageBlockProposal() - update cache (accumulate gas and supply)
			bpF.TotalGasUsed += evmTxFGas
			bpF.TotalSupply = new(big.Int).Add(bpF.TotalSupply, evmTxFSupply)
			bsK1.StageBlockProposal(bpF)

			// [tx end]: FlushBlockProposal()
			// Gas: E+F = 50000+600000 = 650000
			// Supply: 2100 + 5000000 + 60000000 = 65002100
			require.Equal(t, uint64(650000), bpF.TotalGasUsed)
			require.Equal(t, big.NewInt(65002100), bpF.TotalSupply)
			err = bsK1.FlushBlockProposal()
			require.NoError(t, err)

			// --- System chunk tx for block K+1 ---
			bsSystemK1 := environment.NewBlockStore(chainID, backend, backend, backend, root)
			bpCommitK1, err := bsSystemK1.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, uint64(650000), bpCommitK1.TotalGasUsed)      // E+F = 650000
			require.Equal(t, big.NewInt(65002100), bpCommitK1.TotalSupply) // accumulated supply

			err = bsSystemK1.CommitBlockProposal(bpCommitK1)
			require.NoError(t, err)

			// Verify LatestBlock is now block 2 with accumulated values
			latestBlockK1, err := bsSystemK1.LatestBlock()
			require.NoError(t, err)
			require.Equal(t, uint64(2), latestBlockK1.Height)
			require.Equal(t, uint64(650000), latestBlockK1.TotalGasUsed)
			require.Equal(t, big.NewInt(65002100), latestBlockK1.TotalSupply)
			require.Equal(t, prevRandaoBlockK1, latestBlockK1.PrevRandao) // PrevRandao preserved

			// Verify block hashes are correct
			hash0, err := bsSystemK1.BlockHash(0)
			require.NoError(t, err)
			require.Equal(t, types.GenesisBlockHash(chainID), hash0)

			hash1, err := bsSystemK1.BlockHash(1)
			require.NoError(t, err)
			require.Equal(t, committedBlockHash, hash1)

			hash2, err := bsSystemK1.BlockHash(2)
			require.NoError(t, err)
			expectedHash2, err := latestBlockK1.Hash()
			require.NoError(t, err)
			require.Equal(t, expectedHash2, hash2)
		})
	})
}

func TestBlockStore(t *testing.T) {

	var chainID = flow.Testnet
	testutils.RunWithTestBackend(t, chainID, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			bs := environment.NewBlockStore(chainID, backend, backend, backend, root)

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
			bs.StageBlockProposal(bp)
			err = bs.FlushBlockProposal()
			require.NoError(t, err)

			bs = environment.NewBlockStore(chainID, backend, backend, backend, root)
			retbp, err = bs.BlockProposal()
			require.NoError(t, err)
			require.Equal(t, bp, retbp)

			// update the block proposal again
			supply := big.NewInt(100)
			bp.TotalSupply = supply
			bs.StageBlockProposal(bp)
			err = bs.FlushBlockProposal()
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
