package forks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
)

/*****************************************************************************
 * NOTATION:                                                                 *
 * A block is denoted as [(◄<qc_number>) <block_view_number>].               *
 * For example, [(◄1) 2] means: a block of view 2 that has a QC for view 1.  *
 *****************************************************************************/

// TestFinalize_Direct1Chain tests adding a direct 1-chain.
// receives [(◄1) 2] [(◄2) 3]
// it should not finalize any block because there is no finalizable 2-chain.
func TestFinalize_Direct1Chain(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireNoBlocksFinalized(t, forks)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireNoBlocksFinalized(t, forks)
	})
}

// TestFinalize_Direct2Chain tests adding a direct 1-chain on a direct 1-chain (direct 2-chain).
// receives [(◄1) 2] [(◄2) 3] [(◄3) 4]
// it should finalize [(◄1) 2]
func TestFinalize_Direct2Chain(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})
}

// TestFinalize_DirectIndirect2Chain tests adding an indirect 1-chain on a direct 1-chain.
// receives [(◄1) 2] [(◄2) 3] [(◄3) 5]
// it should finalize [(◄1) 2]
func TestFinalize_DirectIndirect2Chain(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})
}

// TestFinalize_IndirectDirect2Chain tests adding a direct 1-chain on an indirect 1-chain.
// receives [(◄1) 2] [(◄2) 4] [(◄4) 5]
// it should not finalize any blocks because there is no finalizable 2-chain.
func TestFinalize_IndirectDirect2Chain(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireNoBlocksFinalized(t, forks)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireNoBlocksFinalized(t, forks)
	})
}

// TestFinalize_Direct2ChainOnIndirect tests adding a direct 2-chain on an indirect 2-chain:
//   - ingesting [(◄1) 3] [(◄3) 5] [(◄5) 6] [(◄6) 7] [(◄7) 8]
//   - should result in finalization of [(◄5) 6]
func TestFinalize_Direct2ChainOnIndirect(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 3)
	builder.Add(3, 5)
	builder.Add(5, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 5, 6)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 5, 6)
	})
}

// TestFinalize_Direct2ChainOnDirect tests adding a sequence of direct 2-chains:
//   - ingesting [(◄1) 2] [(◄2) 3] [(◄3) 4] [(◄4) 5] [(◄5) 6]
//   - should result in finalization of [(◄3) 4]
func TestFinalize_Direct2ChainOnDirect(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 3, 4)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 3, 4)
	})
}

// TestFinalize_Multiple2Chains tests the case where a block can be finalized by different 2-chains.
//   - ingesting [(◄1) 2] [(◄2) 3] [(◄3) 5] [(◄3) 6] [(◄3) 7]
//   - should result in finalization of [(◄1) 2]
func TestFinalize_Multiple2Chains(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 5)
	builder.Add(3, 6)
	builder.Add(3, 7)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})
}

// TestFinalize_OrphanedFork tests that we can finalize a block which causes a conflicting fork to be orphaned.
// We ingest the the following block tree:
//
//	[(◄1) 2] [(◄2) 3]
//	         [(◄2) 4] [(◄4) 5] [(◄5) 6]
//
// which should result in finalization of [(◄2) 4] and pruning of [(◄2) 3]
func TestFinalize_OrphanedFork(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2) // [(◄1) 2]
	builder.Add(2, 3) // [(◄2) 3], should eventually be pruned
	builder.Add(2, 4) // [(◄2) 4]
	builder.Add(4, 5) // [(◄4) 5]
	builder.Add(5, 6) // [(◄5) 6]

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 2, 4)
		require.False(t, forks.IsKnownBlock(blocks[1].Block.BlockID))
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 2, 4)
		require.False(t, forks.IsKnownBlock(blocks[1].Block.BlockID))
	})
}

// TestDuplication tests that delivering the same block/qc multiple times has
// the same end state as delivering the block/qc once.
// receives [(◄1) 2] [(◄2) 3] [(◄2) 3] [(◄3) 4] [(◄3) 4] [(◄4) 5] [(◄4) 5]
// it should finalize [(◄2) 3]
func TestDuplication(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 2, 3)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		require.Nil(t, err)
		requireLatestFinalizedBlock(t, forks, 2, 3)
	})
}

// TestIgnoreBlocksBelowFinalizedView tests that blocks below finalized view are ignored.
// receives [(◄1) 2] [(◄2) 3] [(◄3) 4] [(◄1) 5]
// it should finalize [(◄1) 2]
func TestIgnoreBlocksBelowFinalizedView(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2) // [(◄1) 2]
	builder.Add(2, 3) // [(◄2) 3]
	builder.Add(3, 4) // [(◄3) 4]
	builder.Add(1, 5) // [(◄1) 5]

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		// initialize forks and add first 3 blocks:
		//  * block [(◄1) 2] should then be finalized
		//  * and block [1] should be pruned
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks[:3])
		require.Nil(t, err)
		// sanity checks to confirm correct test setup
		requireLatestFinalizedBlock(t, forks, 1, 2)
		require.False(t, forks.IsKnownBlock(builder.GenesisBlock().ID()))

		// adding block [(◄1) 5]: note that QC is _below_ the pruning threshold, i.e. cannot resolve the parent
		// * Forks should store block, despite the parent already being pruned
		// * finalization should not change
		orphanedBlock := blocks[3].Block
		err = forks.AddProposal(orphanedBlock)
		require.Nil(t, err)
		require.True(t, forks.IsKnownBlock(orphanedBlock.BlockID))
		requireLatestFinalizedBlock(t, forks, 1, 2)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		// initialize forks and add first 3 blocks:
		//  * block [(◄1) 2] should then be finalized
		//  * and block [1] should be pruned
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks[:3])
		require.Nil(t, err)
		// sanity checks to confirm correct test setup
		requireLatestFinalizedBlock(t, forks, 1, 2)
		require.False(t, forks.IsKnownBlock(builder.GenesisBlock().ID()))

		// adding block [(◄1) 5]: note that QC is _below_ the pruning threshold, i.e. cannot resolve the parent
		// * Forks should store block, despite the parent already being pruned
		// * finalization should not change
		certBlockWithUnknownParent := toCertifiedBlock(t, blocks[3].Block)
		err = forks.AddCertifiedBlock(certBlockWithUnknownParent)
		require.Nil(t, err)
		require.True(t, forks.IsKnownBlock(certBlockWithUnknownParent.Block.BlockID))
		requireLatestFinalizedBlock(t, forks, 1, 2)

	})
}

// TestDoubleProposal tests that the DoubleProposal notification is emitted when two different
// proposals for the same view are added. We ingest the the following block tree:
//
//	               / [(◄1) 2]
//			[1]
//	               \ [(◄1) 2']
//
// which should result in a DoubleProposal event referencing the blocks [(◄1) 2] and [(◄1) 2']
func TestDoubleProposal(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)                // [(◄1) 2]
	builder.AddVersioned(1, 2, 0, 1) // [(◄1) 2']

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[1].Block, blocks[0].Block).Once()

		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[1].Block, blocks[0].Block).Once()

		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[0].Block)) // add [(◄1) 2]  as certified block
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1].Block)) // add [(◄1) 2']  as certified block
		require.Nil(t, err)
	})
}

// TestConflictingQCs checks that adding 2 conflicting QCs should return model.ByzantineThresholdExceededError
// We ingest the the following block tree:
//
//	[(◄1) 2] [(◄2) 3]   [(◄3) 4]  [(◄4) 6]
//	         [(◄2) 3']  [(◄3') 5]
//
// which should result in a `ByzantineThresholdExceededError`, because conflicting blocks 3 and 3' both have QCs
func TestConflictingQCs(t *testing.T) {
	builder := NewBlockBuilder()

	builder.Add(1, 2)                // [(◄1) 2]
	builder.Add(2, 3)                // [(◄2) 3]
	builder.AddVersioned(2, 3, 0, 1) // [(◄2) 3']
	builder.Add(3, 4)                // [(◄3) 4]
	builder.Add(4, 6)                // [(◄4) 6]
	builder.AddVersioned(3, 5, 1, 0) // [(◄3') 5]

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Return(nil)

		err = addProposalsToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Return(nil)

		// As [(◄3') 5] is not certified, it will not be added to Forks. However, its QC (◄3') is
		// delivered to Forks as part of the *certified* block [(◄2) 3'].
		err = addCertifiedBlocksToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})
}

// TestConflictingFinalizedForks checks that finalizing 2 conflicting forks should return model.ByzantineThresholdExceededError
// We ingest the the following block tree:
//
//	[(◄1) 2] [(◄2) 3] [(◄3) 4] [(◄4) 5]
//	         [(◄2) 6] [(◄6) 7] [(◄7) 8]
//
// Here, both blocks [(◄2) 3] and [(◄2) 6] satisfy the finalization condition, i.e. we have a fork
// in the finalized blocks, which should result in a model.ByzantineThresholdExceededError exception.
func TestConflictingFinalizedForks(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5) // finalizes [(◄2) 3]
	builder.Add(2, 6)
	builder.Add(6, 7)
	builder.Add(7, 8) // finalizes [(◄2) 6], conflicting with conflicts with [(◄2) 3]

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addProposalsToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})
}

// TestAddUnconnectedProposal checks that adding a proposal which does not connect to the
// latest finalized block returns a `model.MissingBlockError`
//   - receives [(◄2) 3]
//   - should return `model.MissingBlockError`, because the parent is above the pruning
//     threshold, but Forks does not know its parent
func TestAddUnconnectedProposal(t *testing.T) {
	builder := NewBlockBuilder().
		Add(1, 2). // we will skip this block [(◄1) 2]
		Add(2, 3)  // [(◄2) 3]
	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, _ := newForks(t)
		err := forks.AddProposal(blocks[1].Block)
		require.Error(t, err)
		assert.True(t, model.IsMissingBlockError(err))
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err := forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1].Block))
		require.Error(t, err)
		assert.True(t, model.IsMissingBlockError(err))
	})
}

// TestGetProposal tests that we can retrieve stored proposals.
// Attempting to retrieve nonexistent or pruned proposals should fail.
// receives [(◄1) 2] [(◄2) 3] [(◄3) 4], then [(◄4) 5]
// should finalize [(◄1) 2], then [(◄2) 3]
func TestGetProposal(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2) // [(◄1) 2]
	builder.Add(2, 3) // [(◄2) 3]
	builder.Add(3, 4) // [(◄3) 4]
	builder.Add(4, 5) // [(◄4) 5]

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		blocksAddedFirst := blocks[:3]    // [(◄1) 2] [(◄2) 3] [(◄3) 4]
		remainingBlock := blocks[3].Block // [(◄4) 5]
		forks, _ := newForks(t)

		// should be unable to retrieve a block before it is added
		_, ok := forks.GetBlock(blocks[0].Block.BlockID)
		assert.False(t, ok)

		// add first 3 blocks - should finalize [(◄1) 2]
		err = addProposalsToForks(forks, blocksAddedFirst)
		require.Nil(t, err)

		// should be able to retrieve all stored blocks
		for _, proposal := range blocksAddedFirst {
			b, ok := forks.GetBlock(proposal.Block.BlockID)
			assert.True(t, ok)
			assert.Equal(t, proposal.Block, b)
		}

		// add remaining block [(◄4) 5] - should finalize [(◄2) 3] and prune [(◄1) 2]
		require.Nil(t, forks.AddProposal(remainingBlock))

		// should be able to retrieve just added block
		b, ok := forks.GetBlock(remainingBlock.BlockID)
		assert.True(t, ok)
		assert.Equal(t, remainingBlock, b)

		// should be unable to retrieve pruned block
		_, ok = forks.GetBlock(blocksAddedFirst[0].Block.BlockID)
		assert.False(t, ok)
	})

	// Caution: finalization is driven by QCs. Therefore, we include the QC for block 3
	// in the first batch of blocks that we add. This is analogous to previous test case,
	// except that we are delivering the QC (◄3) as part of the certified block of view 2
	//   [(◄2) 3] (◄3)
	// while in the previous sub-test, the QC (◄3) was delivered as part of block [(◄3) 4]
	t.Run("ingest certified blocks", func(t *testing.T) {
		blocksAddedFirst := toCertifiedBlocks(t, toBlocks(blocks[:2])...) // [(◄1) 2] [(◄2) 3] (◄3)
		remainingBlock := toCertifiedBlock(t, blocks[2].Block)            // [(◄3) 4] (◄4)
		forks, _ := newForks(t)

		// should be unable to retrieve a block before it is added
		_, ok := forks.GetBlock(blocks[0].Block.BlockID)
		assert.False(t, ok)

		// add first blocks - should finalize [(◄1) 2]
		err := forks.AddCertifiedBlock(blocksAddedFirst[0])
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(blocksAddedFirst[1])
		require.Nil(t, err)

		// should be able to retrieve all stored blocks
		for _, proposal := range blocksAddedFirst {
			b, ok := forks.GetBlock(proposal.Block.BlockID)
			assert.True(t, ok)
			assert.Equal(t, proposal.Block, b)
		}

		// add remaining block [(◄4) 5] - should finalize [(◄2) 3] and prune [(◄1) 2]
		require.Nil(t, forks.AddCertifiedBlock(remainingBlock))

		// should be able to retrieve just added block
		b, ok := forks.GetBlock(remainingBlock.Block.BlockID)
		assert.True(t, ok)
		assert.Equal(t, remainingBlock.Block, b)

		// should be unable to retrieve pruned block
		_, ok = forks.GetBlock(blocksAddedFirst[0].Block.BlockID)
		assert.False(t, ok)
	})
}

// TestGetProposalsForView tests retrieving proposals for a view (also including double proposals).
// receives [(◄1) 2] [(◄2) 4] [(◄2) 4']
func TestGetProposalsForView(t *testing.T) {

	builder := NewBlockBuilder()
	builder.Add(1, 2)                // [(◄1) 2]
	builder.Add(2, 4)                // [(◄2) 4]
	builder.AddVersioned(2, 4, 0, 1) // [(◄2) 4']

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Once()

		err = addProposalsToForks(forks, blocks)
		require.Nil(t, err)

		// expect 1 proposal at view 2
		proposals := forks.GetBlocksForView(2)
		assert.Len(t, proposals, 1)
		assert.Equal(t, blocks[0].Block, proposals[0])

		// expect 2 proposals at view 4
		proposals = forks.GetBlocksForView(4)
		assert.Len(t, proposals, 2)
		assert.ElementsMatch(t, toBlocks(blocks[1:]), proposals)

		// expect 0 proposals at view 3
		proposals = forks.GetBlocksForView(3)
		assert.Len(t, proposals, 0)
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Once()

		err := forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[0].Block))
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1].Block))
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[2].Block))
		require.Nil(t, err)

		// expect 1 proposal at view 2
		proposals := forks.GetBlocksForView(2)
		assert.Len(t, proposals, 1)
		assert.Equal(t, blocks[0].Block, proposals[0])

		// expect 2 proposals at view 4
		proposals = forks.GetBlocksForView(4)
		assert.Len(t, proposals, 2)
		assert.ElementsMatch(t, toBlocks(blocks[1:]), proposals)

		// expect 0 proposals at view 3
		proposals = forks.GetBlocksForView(3)
		assert.Len(t, proposals, 0)
	})
}

// TestNotification tests that notifier gets correct notifications when incorporating block as well as finalization events.
// receives [(◄1) 2] [(◄2) 3] [(◄3) 4]
// should finalize [(◄1) 2]
func TestNotification(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("ingest proposals", func(t *testing.T) {
		notifier := &mocks.Consumer{}
		// 4 blocks including the genesis are incorporated
		notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(4)
		notifier.On("OnFinalizedBlock", blocks[0].Block).Return(nil).Once()
		finalizationCallback := mockmodule.NewFinalizer(t)
		finalizationCallback.On("MakeFinal", blocks[0].Block.BlockID).Return(nil).Once()

		forks, err := NewForks2(builder.GenesisBlock(), finalizationCallback, notifier)
		require.NoError(t, err)
		require.NoError(t, addProposalsToForks(forks, blocks))
	})

	t.Run("ingest certified blocks", func(t *testing.T) {
		notifier := &mocks.Consumer{}
		// 4 blocks including the genesis are incorporated
		notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(4)
		notifier.On("OnFinalizedBlock", blocks[0].Block).Return(nil).Once()
		finalizationCallback := mockmodule.NewFinalizer(t)
		finalizationCallback.On("MakeFinal", blocks[0].Block.BlockID).Return(nil).Once()

		forks, err := NewForks2(builder.GenesisBlock(), finalizationCallback, notifier)
		require.NoError(t, err)
		require.NoError(t, addCertifiedBlocksToForks(forks, blocks))
	})
}

// ========== internal functions ===============

func newForks(t *testing.T) (*Forks2, *mocks.Consumer) {
	notifier := mocks.NewConsumer(t)
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Maybe()
	notifier.On("OnFinalizedBlock", mock.Anything).Return(nil).Maybe()
	finalizationCallback := mockmodule.NewFinalizer(t)
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil).Maybe()

	genesisBQ := makeGenesis()

	forks, err := NewForks2(genesisBQ, finalizationCallback, notifier)

	require.Nil(t, err)
	return forks, notifier
}

// addProposalsToForks adds all the given blocks to Forks, in order.
// If any errors occur, returns the first one.
func addProposalsToForks(forks *Forks2, proposals []*model.Proposal) error {
	for _, proposal := range proposals {
		err := forks.AddProposal(proposal.Block)
		if err != nil {
			return fmt.Errorf("test failed to add proposal for view %d: %w", proposal.Block.View, err)
		}
	}
	return nil
}

// addCertifiedBlocksToForks iterates over all proposals, caches them locally in a map,
// constructs certified blocks whenever possible and adds the certified blocks to forks,
// If any errors occur, returns the first one.
func addCertifiedBlocksToForks(forks *Forks2, proposals []*model.Proposal) error {
	uncertifiedBlocks := make(map[flow.Identifier]*model.Block)
	for _, proposal := range proposals {
		uncertifiedBlocks[proposal.Block.BlockID] = proposal.Block
		parentID := proposal.Block.QC.BlockID
		parent, found := uncertifiedBlocks[parentID]
		if !found {
			continue
		}
		delete(uncertifiedBlocks, parentID)

		certParent, err := model.NewCertifiedBlock(parent, proposal.Block.QC)
		if err != nil {
			return fmt.Errorf("test failed to creat certified block for view %d: %w", certParent.Block.View, err)
		}
		err = forks.AddCertifiedBlock(&certParent)
		if err != nil {
			return fmt.Errorf("test failed to add certified block for view %d: %w", certParent.Block.View, err)
		}
	}

	return nil
}

// requireLatestFinalizedBlock asserts that the latest finalized block has the given view and qc view.
func requireLatestFinalizedBlock(t *testing.T, forks *Forks2, qcView int, view int) {
	require.Equal(t, forks.FinalizedBlock().View, uint64(view), "finalized block has wrong view")
	require.Equal(t, forks.FinalizedBlock().QC.View, uint64(qcView), "finalized block has wrong qc")
}

// requireNoBlocksFinalized asserts that no blocks have been finalized (genesis is latest finalized block).
func requireNoBlocksFinalized(t *testing.T, forks *Forks2) {
	genesis := makeGenesis()
	require.Equal(t, forks.FinalizedBlock().View, genesis.Block.View)
	require.Equal(t, forks.FinalizedBlock().View, genesis.CertifyingQC.View)
}

// toBlocks converts the given proposals to slice of blocks
// TODO: change `BlockBuilder` to generate model.Blocks instead of model.Proposals and then remove this method
func toBlocks(proposals []*model.Proposal) []*model.Block {
	blocks := make([]*model.Block, 0, len(proposals))
	for _, b := range proposals {
		blocks = append(blocks, b.Block)
	}
	return blocks
}

// toCertifiedBlock generates a QC for the given block and returns their combination as a certified block
func toCertifiedBlock(t *testing.T, block *model.Block) *model.CertifiedBlock {
	qc := &flow.QuorumCertificate{
		View:    block.View,
		BlockID: block.BlockID,
	}
	cb, err := model.NewCertifiedBlock(block, qc)
	require.Nil(t, err)
	return &cb
}

// toCertifiedBlocks generates a QC for the given block and returns their combination as a certified blocks
func toCertifiedBlocks(t *testing.T, blocks ...*model.Block) []*model.CertifiedBlock {
	certBlocks := make([]*model.CertifiedBlock, 0, len(blocks))
	for _, b := range blocks {
		certBlocks = append(certBlocks, toCertifiedBlock(t, b))
	}
	return certBlocks
}
