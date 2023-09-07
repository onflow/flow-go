package forks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
)

/*****************************************************************************
 * NOTATION:                                                                 *
 * A block is denoted as [◄(<qc_number>) <block_view_number>].               *
 * For example, [◄(1) 2] means: a block of view 2 that has a QC for view 1.  *
 *****************************************************************************/

// TestInitialization verifies that at initialization, Forks reports:
//   - the root / genesis block as finalized
//   - it has no finalization proof for the root / genesis block (block and its finalization is trusted)
func TestInitialization(t *testing.T) {
	forks, _ := newForks(t)
	requireOnlyGenesisBlockFinalized(t, forks)
	_, hasProof := forks.FinalityProof()
	require.False(t, hasProof)
}

// TestFinalize_Direct1Chain tests adding a direct 1-chain on top of the genesis block:
//   - receives [◄(1) 2] [◄(2) 5]
//
// Expected behaviour:
//   - On the one hand, Forks should not finalize any _additional_ blocks, because there is
//     no finalizable 2-chain for [◄(1) 2]. Hence, finalization no events should be emitted.
//   - On the other hand, after adding the two blocks, Forks has enough knowledge to construct
//     a FinalityProof for the genesis block.
func TestFinalize_Direct1Chain(t *testing.T) {
	builder := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3)
	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)

		// adding block [◄(1) 2] should not finalize anything
		// as the genesis block is trusted, there should be no FinalityProof available for it
		require.NoError(t, forks.AddValidatedBlock(blocks[0]))
		requireOnlyGenesisBlockFinalized(t, forks)
		_, hasProof := forks.FinalityProof()
		require.False(t, hasProof)

		// After adding block [◄(2) 3], Forks has enough knowledge to construct a FinalityProof for the
		// genesis block. However, finalization remains at the genesis block, so no events should be emitted.
		expectedFinalityProof := makeFinalityProof(t, builder.GenesisBlock().Block, blocks[0], blocks[1].QC)
		require.NoError(t, forks.AddValidatedBlock(blocks[1]))
		requireLatestFinalizedBlock(t, forks, builder.GenesisBlock().Block)
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)

		// After adding CertifiedBlock [◄(1) 2] ◄(2), Forks has enough knowledge to construct a FinalityProof for
		// the genesis block. However, finalization remains at the genesis block, so no events should be emitted.
		expectedFinalityProof := makeFinalityProof(t, builder.GenesisBlock().Block, blocks[0], blocks[1].QC)
		c, err := model.NewCertifiedBlock(blocks[0], blocks[1].QC)
		require.NoError(t, err)

		require.NoError(t, forks.AddCertifiedBlock(&c))
		requireLatestFinalizedBlock(t, forks, builder.GenesisBlock().Block)
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_Direct2Chain tests adding a direct 1-chain on a direct 1-chain (direct 2-chain).
//   - receives [◄(1) 2] [◄(2) 3] [◄(3) 4]
//   - Forks should finalize [◄(1) 2]
func TestFinalize_Direct2Chain(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 4).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[0], blocks[1], blocks[2].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_DirectIndirect2Chain tests adding an indirect 1-chain on a direct 1-chain.
// receives [◄(1) 2] [◄(2) 3] [◄(3) 5]
// it should finalize [◄(1) 2]
func TestFinalize_DirectIndirect2Chain(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 5).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[0], blocks[1], blocks[2].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_IndirectDirect2Chain tests adding a direct 1-chain on an indirect 1-chain.
//   - Forks receives [◄(1) 3] [◄(3) 5] [◄(7) 7]
//   - it should not finalize any blocks because there is no finalizable 2-chain.
func TestFinalize_IndirectDirect2Chain(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 3).
		Add(3, 5).
		Add(5, 7).
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireOnlyGenesisBlockFinalized(t, forks)
		_, hasProof := forks.FinalityProof()
		require.False(t, hasProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireOnlyGenesisBlockFinalized(t, forks)
		_, hasProof := forks.FinalityProof()
		require.False(t, hasProof)
	})
}

// TestFinalize_Direct2ChainOnIndirect tests adding a direct 2-chain on an indirect 2-chain:
//   - ingesting [◄(1) 3] [◄(3) 5] [◄(5) 6] [◄(6) 7] [◄(7) 8]
//   - should result in finalization of [◄(5) 6]
func TestFinalize_Direct2ChainOnIndirect(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 3).
		Add(3, 5).
		Add(5, 6).
		Add(6, 7).
		Add(7, 8).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[2], blocks[3], blocks[4].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_Direct2ChainOnDirect tests adding a sequence of direct 2-chains:
//   - ingesting [◄(1) 2] [◄(2) 3] [◄(3) 4] [◄(4) 5] [◄(5) 6]
//   - should result in finalization of [◄(3) 4]
func TestFinalize_Direct2ChainOnDirect(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 4).
		Add(4, 5).
		Add(5, 6).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[2], blocks[3], blocks[4].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_Multiple2Chains tests the case where a block can be finalized by different 2-chains.
//   - ingesting [◄(1) 2] [◄(2) 3] [◄(3) 5] [◄(3) 6] [◄(3) 7]
//   - should result in finalization of [◄(1) 2]
func TestFinalize_Multiple2Chains(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 5).
		Add(3, 6).
		Add(3, 7).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[0], blocks[1], blocks[2].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestFinalize_OrphanedFork tests that we can finalize a block which causes a conflicting fork to be orphaned.
// We ingest the the following block tree:
//
//	[◄(1) 2] [◄(2) 3]
//	         [◄(2) 4] [◄(4) 5] [◄(5) 6]
//
// which should result in finalization of [◄(2) 4] and pruning of [◄(2) 3]
func TestFinalize_OrphanedFork(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2). // [◄(1) 2]
		Add(2, 3). // [◄(2) 3], should eventually be pruned
		Add(2, 4). // [◄(2) 4], should eventually be finalized
		Add(4, 5). // [◄(4) 5]
		Add(5, 6). // [◄(5) 6]
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[2], blocks[3], blocks[4].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		require.False(t, forks.IsKnownBlock(blocks[1].BlockID))
		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		require.False(t, forks.IsKnownBlock(blocks[1].BlockID))
		requireLatestFinalizedBlock(t, forks, blocks[2])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestDuplication tests that delivering the same block/qc multiple times has
// the same end state as delivering the block/qc once.
//   - Forks receives [◄(1) 2] [◄(2) 3] [◄(2) 3] [◄(3) 4] [◄(3) 4] [◄(4) 5] [◄(4) 5]
//   - it should finalize [◄(2) 3]
func TestDuplication(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(2, 3).
		Add(3, 4).
		Add(3, 4).
		Add(4, 5).
		Add(4, 5).
		Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[1], blocks[3], blocks[5].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[1])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks))

		requireLatestFinalizedBlock(t, forks, blocks[1])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestIgnoreBlocksBelowFinalizedView tests that blocks below finalized view are ignored.
//   - Forks receives [◄(1) 2] [◄(2) 3] [◄(3) 4] [◄(1) 5]
//   - it should finalize [◄(1) 2]
func TestIgnoreBlocksBelowFinalizedView(t *testing.T) {
	builder := NewBlockBuilder().
		Add(1, 2). // [◄(1) 2]
		Add(2, 3). // [◄(2) 3]
		Add(3, 4). // [◄(3) 4]
		Add(1, 5)  // [◄(1) 5]
	blocks, err := builder.Blocks()
	require.Nil(t, err)
	expectedFinalityProof := makeFinalityProof(t, blocks[0], blocks[1], blocks[2].QC)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		// initialize forks and add first 3 blocks:
		//  * block [◄(1) 2] should then be finalized
		//  * and block [1] should be pruned
		forks, _ := newForks(t)
		require.Nil(t, addValidatedBlockToForks(forks, blocks[:3]))

		// sanity checks to confirm correct test setup
		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
		require.False(t, forks.IsKnownBlock(builder.GenesisBlock().ID()))

		// adding block [◄(1) 5]: note that QC is _below_ the pruning threshold, i.e. cannot resolve the parent
		// * Forks should store block, despite the parent already being pruned
		// * finalization should not change
		orphanedBlock := blocks[3]
		require.Nil(t, forks.AddValidatedBlock(orphanedBlock))
		require.True(t, forks.IsKnownBlock(orphanedBlock.BlockID))
		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		// initialize forks and add first 3 blocks:
		//  * block [◄(1) 2] should then be finalized
		//  * and block [1] should be pruned
		forks, _ := newForks(t)
		require.Nil(t, addCertifiedBlocksToForks(forks, blocks[:3]))
		// sanity checks to confirm correct test setup
		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
		require.False(t, forks.IsKnownBlock(builder.GenesisBlock().ID()))

		// adding block [◄(1) 5]: note that QC is _below_ the pruning threshold, i.e. cannot resolve the parent
		// * Forks should store block, despite the parent already being pruned
		// * finalization should not change
		certBlockWithUnknownParent := toCertifiedBlock(t, blocks[3])
		require.Nil(t, forks.AddCertifiedBlock(certBlockWithUnknownParent))
		require.True(t, forks.IsKnownBlock(certBlockWithUnknownParent.Block.BlockID))
		requireLatestFinalizedBlock(t, forks, blocks[0])
		requireFinalityProof(t, forks, expectedFinalityProof)
	})
}

// TestDoubleProposal tests that the DoubleProposal notification is emitted when two different
// blocks for the same view are added. We ingest the the following block tree:
//
//	               / [◄(1) 2]
//			[1]
//	               \ [◄(1) 2']
//
// which should result in a DoubleProposal event referencing the blocks [◄(1) 2] and [◄(1) 2']
func TestDoubleProposal(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).                // [◄(1) 2]
		AddVersioned(1, 2, 0, 1). // [◄(1) 2']
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[1], blocks[0]).Once()

		err = addValidatedBlockToForks(forks, blocks)
		require.Nil(t, err)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[1], blocks[0]).Once()

		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[0])) // add [◄(1) 2]  as certified block
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1])) // add [◄(1) 2']  as certified block
		require.Nil(t, err)
	})
}

// TestConflictingQCs checks that adding 2 conflicting QCs should return model.ByzantineThresholdExceededError
// We ingest the following block tree:
//
//	[◄(1) 2] [◄(2) 3]   [◄(3) 4]  [◄(4) 6]
//	         [◄(2) 3']  [◄(3') 5]
//
// which should result in a `ByzantineThresholdExceededError`, because conflicting blocks 3 and 3' both have QCs
func TestConflictingQCs(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).                // [◄(1) 2]
		Add(2, 3).                // [◄(2) 3]
		AddVersioned(2, 3, 0, 1). // [◄(2) 3']
		Add(3, 4).                // [◄(3) 4]
		Add(4, 6).                // [◄(4) 6]
		AddVersioned(3, 5, 1, 0). // [◄(3') 5]
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2], blocks[1]).Return(nil)

		err = addValidatedBlockToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2], blocks[1]).Return(nil)

		// As [◄(3') 5] is not certified, it will not be added to Forks. However, its QC ◄(3') is
		// delivered to Forks as part of the *certified* block [◄(2) 3'].
		err = addCertifiedBlocksToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})
}

// TestConflictingFinalizedForks checks that finalizing 2 conflicting forks should return model.ByzantineThresholdExceededError
// We ingest the the following block tree:
//
//	[◄(1) 2] [◄(2) 3] [◄(3) 4] [◄(4) 5]
//	         [◄(2) 6] [◄(6) 7] [◄(7) 8]
//
// Here, both blocks [◄(2) 3] and [◄(2) 6] satisfy the finalization condition, i.e. we have a fork
// in the finalized blocks, which should result in a model.ByzantineThresholdExceededError exception.
func TestConflictingFinalizedForks(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 4).
		Add(4, 5). // finalizes [◄(2) 3]
		Add(2, 6).
		Add(6, 7).
		Add(7, 8). // finalizes [◄(2) 6], conflicting with conflicts with [◄(2) 3]
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addValidatedBlockToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err = addCertifiedBlocksToForks(forks, blocks)
		assert.True(t, model.IsByzantineThresholdExceededError(err))
	})
}

// TestAddDisconnectedBlock checks that adding a block which does not connect to the
// latest finalized block returns a `model.MissingBlockError`
//   - receives [◄(2) 3]
//   - should return `model.MissingBlockError`, because the parent is above the pruning
//     threshold, but Forks does not know its parent
func TestAddDisconnectedBlock(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2). // we will skip this block [◄(1) 2]
		Add(2, 3). // [◄(2) 3]
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err := forks.AddValidatedBlock(blocks[1])
		require.Error(t, err)
		assert.True(t, model.IsMissingBlockError(err))
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, _ := newForks(t)
		err := forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1]))
		require.Error(t, err)
		assert.True(t, model.IsMissingBlockError(err))
	})
}

// TestGetBlock tests that we can retrieve stored blocks. Here, we test that
// attempting to retrieve nonexistent or pruned blocks fails without causing an exception.
//   - Forks receives [◄(1) 2] [◄(2) 3] [◄(3) 4], then [◄(4) 5]
//   - should finalize [◄(1) 2], then [◄(2) 3]
func TestGetBlock(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2). // [◄(1) 2]
		Add(2, 3). // [◄(2) 3]
		Add(3, 4). // [◄(3) 4]
		Add(4, 5). // [◄(4) 5]
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		blocksAddedFirst := blocks[:3] // [◄(1) 2] [◄(2) 3] [◄(3) 4]
		remainingBlock := blocks[3]    // [◄(4) 5]
		forks, _ := newForks(t)

		// should be unable to retrieve a block before it is added
		_, ok := forks.GetBlock(blocks[0].BlockID)
		assert.False(t, ok)

		// add first 3 blocks - should finalize [◄(1) 2]
		err = addValidatedBlockToForks(forks, blocksAddedFirst)
		require.Nil(t, err)

		// should be able to retrieve all stored blocks
		for _, block := range blocksAddedFirst {
			b, ok := forks.GetBlock(block.BlockID)
			assert.True(t, ok)
			assert.Equal(t, block, b)
		}

		// add remaining block [◄(4) 5] - should finalize [◄(2) 3] and prune [◄(1) 2]
		require.Nil(t, forks.AddValidatedBlock(remainingBlock))

		// should be able to retrieve just added block
		b, ok := forks.GetBlock(remainingBlock.BlockID)
		assert.True(t, ok)
		assert.Equal(t, remainingBlock, b)

		// should be unable to retrieve pruned block
		_, ok = forks.GetBlock(blocksAddedFirst[0].BlockID)
		assert.False(t, ok)
	})

	// Caution: finalization is driven by QCs. Therefore, we include the QC for block 3
	// in the first batch of blocks that we add. This is analogous to previous test case,
	// except that we are delivering the QC ◄(3) as part of the certified block of view 2
	//   [◄(2) 3] ◄(3)
	// while in the previous sub-test, the QC ◄(3) was delivered as part of block [◄(3) 4]
	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		blocksAddedFirst := toCertifiedBlocks(t, blocks[:2]...) // [◄(1) 2] [◄(2) 3] ◄(3)
		remainingBlock := toCertifiedBlock(t, blocks[2])        // [◄(3) 4] ◄(4)
		forks, _ := newForks(t)

		// should be unable to retrieve a block before it is added
		_, ok := forks.GetBlock(blocks[0].BlockID)
		assert.False(t, ok)

		// add first blocks - should finalize [◄(1) 2]
		err := forks.AddCertifiedBlock(blocksAddedFirst[0])
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(blocksAddedFirst[1])
		require.Nil(t, err)

		// should be able to retrieve all stored blocks
		for _, block := range blocksAddedFirst {
			b, ok := forks.GetBlock(block.Block.BlockID)
			assert.True(t, ok)
			assert.Equal(t, block.Block, b)
		}

		// add remaining block [◄(4) 5] - should finalize [◄(2) 3] and prune [◄(1) 2]
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

// TestGetBlocksForView tests retrieving blocks for a view (also including double proposals).
//   - Forks receives [◄(1) 2] [◄(2) 4] [◄(2) 4'],
//     where [◄(2) 4'] is a double proposal, because it has the same view as [◄(2) 4]
//
// Expected behaviour:
//   - Forks should store all the blocks
//   - Forks should emit a `OnDoubleProposeDetected` notification
//   - we can retrieve all blocks, including the double proposals
func TestGetBlocksForView(t *testing.T) {
	blocks, err := NewBlockBuilder().
		Add(1, 2).                // [◄(1) 2]
		Add(2, 4).                // [◄(2) 4]
		AddVersioned(2, 4, 0, 1). // [◄(2) 4']
		Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2], blocks[1]).Once()

		err = addValidatedBlockToForks(forks, blocks)
		require.Nil(t, err)

		// expect 1 block at view 2
		storedBlocks := forks.GetBlocksForView(2)
		assert.Len(t, storedBlocks, 1)
		assert.Equal(t, blocks[0], storedBlocks[0])

		// expect 2 blocks at view 4
		storedBlocks = forks.GetBlocksForView(4)
		assert.Len(t, storedBlocks, 2)
		assert.ElementsMatch(t, blocks[1:], storedBlocks)

		// expect 0 blocks at view 3
		storedBlocks = forks.GetBlocksForView(3)
		assert.Len(t, storedBlocks, 0)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, notifier := newForks(t)
		notifier.On("OnDoubleProposeDetected", blocks[2], blocks[1]).Once()

		err := forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[0]))
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1]))
		require.Nil(t, err)
		err = forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[2]))
		require.Nil(t, err)

		// expect 1 block at view 2
		storedBlocks := forks.GetBlocksForView(2)
		assert.Len(t, storedBlocks, 1)
		assert.Equal(t, blocks[0], storedBlocks[0])

		// expect 2 blocks at view 4
		storedBlocks = forks.GetBlocksForView(4)
		assert.Len(t, storedBlocks, 2)
		assert.ElementsMatch(t, blocks[1:], storedBlocks)

		// expect 0 blocks at view 3
		storedBlocks = forks.GetBlocksForView(3)
		assert.Len(t, storedBlocks, 0)
	})
}

// TestNotifications tests that Forks emits the expected events:
//   - Forks receives [◄(1) 2] [◄(2) 3] [◄(3) 4]
//
// Expected Behaviour:
//   - Each of the ingested blocks should result in an `OnBlockIncorporated` notification
//   - Forks should finalize [◄(1) 2], resulting in a `MakeFinal` event and an `OnFinalizedBlock` event
func TestNotifications(t *testing.T) {
	builder := NewBlockBuilder().
		Add(1, 2).
		Add(2, 3).
		Add(3, 4)
	blocks, err := builder.Blocks()
	require.Nil(t, err)

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		notifier := &mocks.Consumer{}
		// 4 blocks including the genesis are incorporated
		notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(4)
		notifier.On("OnFinalizedBlock", blocks[0]).Once()
		finalizationCallback := mockmodule.NewFinalizer(t)
		finalizationCallback.On("MakeFinal", blocks[0].BlockID).Return(nil).Once()

		forks, err := New(builder.GenesisBlock(), finalizationCallback, notifier)
		require.NoError(t, err)
		require.NoError(t, addValidatedBlockToForks(forks, blocks))
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		notifier := &mocks.Consumer{}
		// 4 blocks including the genesis are incorporated
		notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(4)
		notifier.On("OnFinalizedBlock", blocks[0]).Once()
		finalizationCallback := mockmodule.NewFinalizer(t)
		finalizationCallback.On("MakeFinal", blocks[0].BlockID).Return(nil).Once()

		forks, err := New(builder.GenesisBlock(), finalizationCallback, notifier)
		require.NoError(t, err)
		require.NoError(t, addCertifiedBlocksToForks(forks, blocks))
	})
}

// TestFinalizingMultipleBlocks tests that `OnFinalizedBlock` notifications are emitted in correct order
// when there are multiple blocks finalized by adding a _single_ block.
//   - receiving [◄(1) 3] [◄(3) 5] [◄(5) 7] [◄(7) 11] [◄(11) 12] should not finalize any blocks,
//     because there is no 2-chain with the first chain link being a _direct_ 1-chain
//   - adding [◄(12) 22] should finalize up to block [◄(6) 11]
//
// This test verifies the following expected properties:
//  1. Safety under reentrancy:
//     While Forks is single-threaded, there is still the possibility of reentrancy. Specifically, the
//     consumers of our finalization events are served by the goroutine executing Forks. It is conceivable
//     that a consumer might access Forks and query the latest finalization proof. This would be legal, if
//     the component supplying the goroutine to Forks also consumes the notifications. Therefore, for API
//     safety, we require forks to _first update_ its `FinalityProof()` before it emits _any_ events.
//  2. For each finalized block, `finalizationCallback` event is executed _before_ `OnFinalizedBlock` notifications.
//  3. Blocks are finalized in order of increasing height (without skipping any blocks).
func TestFinalizingMultipleBlocks(t *testing.T) {
	builder := NewBlockBuilder().
		Add(1, 3).   // index 0: [◄(1) 2]
		Add(3, 5).   // index 1: [◄(2) 4]
		Add(5, 7).   // index 2: [◄(4) 6]
		Add(7, 11).  // index 3: [◄(6) 11] -- expected to be finalized
		Add(11, 12). // index 4: [◄(11) 12]
		Add(12, 22)  // index 5: [◄(12) 22]
	blocks, err := builder.Blocks()
	require.Nil(t, err)

	// The Finality Proof should right away point to the _latest_ finalized block. Subsequently emitting
	// Finalization events for lower blocks is fine, because notifications are guaranteed to be
	// _eventually_ arriving. I.e. consumers expect notifications / events to be potentially lagging behind.
	expectedFinalityProof := makeFinalityProof(t, blocks[3], blocks[4], blocks[5].QC)

	setupForksAndAssertions := func() (*Forks, *mockmodule.Finalizer, *mocks.Consumer) {
		// initialize Forks with custom event consumers so we can check order of emitted events
		notifier := &mocks.Consumer{}
		finalizationCallback := mockmodule.NewFinalizer(t)
		notifier.On("OnBlockIncorporated", mock.Anything).Return(nil)
		forks, err := New(builder.GenesisBlock(), finalizationCallback, notifier)
		require.NoError(t, err)

		// expecting finalization of [◄(1) 2] [◄(2) 4] [◄(4) 6] [◄(6) 11] in this order
		blocksAwaitingFinalization := toBlockAwaitingFinalization(blocks[:4])

		finalizationCallback.On("MakeFinal", mock.Anything).Run(func(args mock.Arguments) {
			requireFinalityProof(t, forks, expectedFinalityProof) // Requirement 1: forks should _first update_ its `FinalityProof()` before it emits _any_ events

			// Requirement 3: finalized in order of increasing height (without skipping any blocks).
			expectedNextFinalizationEvents := blocksAwaitingFinalization[0]
			require.Equal(t, expectedNextFinalizationEvents.Block.BlockID, args[0])

			// Requirement 2: finalized block, `finalizationCallback` event is executed _before_ `OnFinalizedBlock` notifications.
			// no duplication of events under normal operations expected
			require.False(t, expectedNextFinalizationEvents.MakeFinalCalled)
			require.False(t, expectedNextFinalizationEvents.OnFinalizedBlockEmitted)
			expectedNextFinalizationEvents.MakeFinalCalled = true
		}).Return(nil).Times(4)

		notifier.On("OnFinalizedBlock", mock.Anything).Run(func(args mock.Arguments) {
			requireFinalityProof(t, forks, expectedFinalityProof) // Requirement 1: forks should _first update_ its `FinalityProof()` before it emits _any_ events

			// Requirement 3: finalized in order of increasing height (without skipping any blocks).
			expectedNextFinalizationEvents := blocksAwaitingFinalization[0]
			require.Equal(t, expectedNextFinalizationEvents.Block, args[0])

			// Requirement 2: finalized block, `finalizationCallback` event is executed _before_ `OnFinalizedBlock` notifications.
			// no duplication of events under normal operations expected
			require.True(t, expectedNextFinalizationEvents.MakeFinalCalled)
			require.False(t, expectedNextFinalizationEvents.OnFinalizedBlockEmitted)
			expectedNextFinalizationEvents.OnFinalizedBlockEmitted = true

			// At this point, `MakeFinal` and `OnFinalizedBlock` have both been emitted for the block, so we are done with it
			blocksAwaitingFinalization = blocksAwaitingFinalization[1:]
		}).Times(4)

		return forks, finalizationCallback, notifier
	}

	t.Run("consensus participant mode: ingest validated blocks", func(t *testing.T) {
		forks, finalizationCallback, notifier := setupForksAndAssertions()
		err = addValidatedBlockToForks(forks, blocks[:5]) // adding [◄(1) 2] [◄(2) 4] [◄(4) 6] [◄(6) 11] [◄(11) 12]
		require.Nil(t, err)
		requireOnlyGenesisBlockFinalized(t, forks) // finalization should still be at the genesis block

		require.NoError(t, forks.AddValidatedBlock(blocks[5])) // adding [◄(12) 22] should trigger finalization events
		requireFinalityProof(t, forks, expectedFinalityProof)
		finalizationCallback.AssertExpectations(t)
		notifier.AssertExpectations(t)
	})

	t.Run("consensus follower mode: ingest certified blocks", func(t *testing.T) {
		forks, finalizationCallback, notifier := setupForksAndAssertions()
		// adding [◄(1) 2] [◄(2) 4] [◄(4) 6] [◄(6) 11] ◄(11)
		require.NoError(t, forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[0])))
		require.NoError(t, forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[1])))
		require.NoError(t, forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[2])))
		require.NoError(t, forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[3])))
		require.Nil(t, err)
		requireOnlyGenesisBlockFinalized(t, forks) // finalization should still be at the genesis block

		// adding certified block [◄(11) 12] ◄(12) should trigger finalization events
		require.NoError(t, forks.AddCertifiedBlock(toCertifiedBlock(t, blocks[4])))
		requireFinalityProof(t, forks, expectedFinalityProof)
		finalizationCallback.AssertExpectations(t)
		notifier.AssertExpectations(t)
	})
}

//* ************************************* internal functions ************************************* */

func newForks(t *testing.T) (*Forks, *mocks.Consumer) {
	notifier := mocks.NewConsumer(t)
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Maybe()
	notifier.On("OnFinalizedBlock", mock.Anything).Maybe()
	finalizationCallback := mockmodule.NewFinalizer(t)
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil).Maybe()

	genesisBQ := makeGenesis()

	forks, err := New(genesisBQ, finalizationCallback, notifier)

	require.Nil(t, err)
	return forks, notifier
}

// addValidatedBlockToForks adds all the given blocks to Forks, in order.
// If any errors occur, returns the first one.
func addValidatedBlockToForks(forks *Forks, blocks []*model.Block) error {
	for _, block := range blocks {
		err := forks.AddValidatedBlock(block)
		if err != nil {
			return fmt.Errorf("test failed to add block for view %d: %w", block.View, err)
		}
	}
	return nil
}

// addCertifiedBlocksToForks iterates over all blocks, caches them locally in a map,
// constructs certified blocks whenever possible and adds the certified blocks to forks,
// Note: if blocks is a single fork, the _last block_ in the slice will not be added,
//
//	because there is no qc for it
//
// If any errors occur, returns the first one.
func addCertifiedBlocksToForks(forks *Forks, blocks []*model.Block) error {
	uncertifiedBlocks := make(map[flow.Identifier]*model.Block)
	for _, b := range blocks {
		uncertifiedBlocks[b.BlockID] = b
		parentID := b.QC.BlockID
		parent, found := uncertifiedBlocks[parentID]
		if !found {
			continue
		}
		delete(uncertifiedBlocks, parentID)

		certParent, err := model.NewCertifiedBlock(parent, b.QC)
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
func requireLatestFinalizedBlock(t *testing.T, forks *Forks, expectedFinalized *model.Block) {
	require.Equal(t, expectedFinalized, forks.FinalizedBlock(), "finalized block is not as expected")
	require.Equal(t, forks.FinalizedView(), expectedFinalized.View, "FinalizedView returned wrong value")
}

// requireOnlyGenesisBlockFinalized asserts that no blocks have been finalized beyond the genesis block.
// Caution: does not inspect output of `forks.FinalityProof()`
func requireOnlyGenesisBlockFinalized(t *testing.T, forks *Forks) {
	genesis := makeGenesis()
	require.Equal(t, forks.FinalizedBlock(), genesis.Block, "finalized block is not the genesis block")
	require.Equal(t, forks.FinalizedBlock().View, genesis.Block.View)
	require.Equal(t, forks.FinalizedBlock().View, genesis.CertifyingQC.View)
	require.Equal(t, forks.FinalizedView(), genesis.Block.View, "finalized block has wrong qc")

	finalityProof, isKnown := forks.FinalityProof()
	require.Nil(t, finalityProof, "expecting finality proof to be nil for genesis block at initialization")
	require.False(t, isKnown, "no finality proof should be known for genesis block at initialization")
}

// requireNoBlocksFinalized asserts that no blocks have been finalized (genesis is latest finalized block).
func requireFinalityProof(t *testing.T, forks *Forks, expectedFinalityProof *hotstuff.FinalityProof) {
	finalityProof, isKnown := forks.FinalityProof()
	require.True(t, isKnown)
	require.Equal(t, expectedFinalityProof, finalityProof)
	require.Equal(t, forks.FinalizedBlock(), expectedFinalityProof.Block)
	require.Equal(t, forks.FinalizedView(), expectedFinalityProof.Block.View)
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

func makeFinalityProof(t *testing.T, block *model.Block, directChild *model.Block, qcCertifyingChild *flow.QuorumCertificate) *hotstuff.FinalityProof {
	c, err := model.NewCertifiedBlock(directChild, qcCertifyingChild) // certified child of FinalizedBlock
	require.NoError(t, err)
	return &hotstuff.FinalityProof{Block: block, CertifiedChild: c}
}

// blockAwaitingFinalization is intended for tracking finalization events and their order for a specific block
type blockAwaitingFinalization struct {
	Block                   *model.Block
	MakeFinalCalled         bool // indicates whether `Finalizer.MakeFinal` was called
	OnFinalizedBlockEmitted bool // indicates whether `OnFinalizedBlockCalled` notification was emitted
}

// toBlockAwaitingFinalization creates a `blockAwaitingFinalization` tracker for each input block
func toBlockAwaitingFinalization(blocks []*model.Block) []*blockAwaitingFinalization {
	trackers := make([]*blockAwaitingFinalization, 0, len(blocks))
	for _, b := range blocks {
		tracker := &blockAwaitingFinalization{b, false, false}
		trackers = append(trackers, tracker)
	}
	return trackers
}
