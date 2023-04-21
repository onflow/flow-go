package forks_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	mockmodule "github.com/onflow/flow-go/module/mock"
)

/* ***************************************************************************************************
 *  TO BE REMOVED: I have moved the tests for the prior version of Forks to this file for reference.
 *************************************************************************************************** */

// NOTATION:
// A block is denoted as [<qc_number>, <block_view_number>].
// For example, [1,2] means: a block of view 2 has a QC for view 1.

// TestFinalize_Direct1Chain tests adding a direct 1-chain.
// receives [1,2] [2,3]
// it should not finalize any block because there is no finalizable 2-chain.
func TestFinalize_Direct1Chain(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireNoBlocksFinalized(t, forks)
}

// TestFinalize_Direct2Chain tests adding a direct 1-chain on a direct 1-chain (direct 2-chain).
// receives [1,2] [2,3] [3,4]
// it should finalize [1,2]
func TestFinalize_Direct2Chain(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// TestFinalize_DirectIndirect2Chain tests adding an indirect 1-chain on a direct 1-chain.
// receives [1,2] [2,3] [3,5]
// it should finalize [1,2]
func TestFinalize_DirectIndirect2Chain(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// TestFinalize_IndirectDirect2Chain tests adding a direct 1-chain on an indirect 1-chain.
// receives [1,2] [2,4] [4,5]
// it should not finalize any blocks because there is no finalizable 2-chain.
func TestFinalize_IndirectDirect2Chain(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireNoBlocksFinalized(t, forks)
}

// TestFinalize_Direct2ChainOnIndirect tests adding a direct 2-chain on an indirect 2-chain.
// The head of highest 2-chain should be finalized.
// receives [1,3] [3,5] [5,6] [6,7] [7,8]
// it should finalize [5,6]
func TestFinalize_Direct2ChainOnIndirect(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 3)
	builder.Add(3, 5)
	builder.Add(5, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 5, 6)
}

// TestFinalize_Direct2ChainOnDirect tests adding a sequence of direct 2-chains.
// The head of highest 2-chain should be finalized.
// receives [1,2] [2,3] [3,4] [4,5] [5,6]
// it should finalize [3,4]
func TestFinalize_Direct2ChainOnDirect(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 3, 4)
}

// TestFinalize_Multiple2Chains tests the case where a block can be finalized
// by different 2-chains.
// receives [1,2] [2,3] [3,5] [3,6] [3,7]
// it should finalize [1,2]
func TestFinalize_Multiple2Chains(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 5)
	builder.Add(3, 6)
	builder.Add(3, 7)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// TestFinalize_OrphanedFork tests that we can finalize a block which causes
// a conflicting fork to be orphaned.
// receives [1,2] [2,3] [2,4] [4,5] [5,6]
// it should finalize [2,4]
func TestFinalize_OrphanedFork(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(2, 4)
	builder.Add(4, 5)
	builder.Add(5, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 4)
}

// TestDuplication tests that delivering the same block/qc multiple times has
// the same end state as delivering the block/qc once.
// receives [1,2] [2,3] [2,3] [3,4] [3,4] [4,5] [4,5]
// it should finalize [2,3]
func TestDuplication(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
}

// TestIgnoreBlocksBelowFinalizedView tests that blocks below finalized view are ignored.
// receives [1,2] [2,3] [3,4] [1,5]
// it should finalize [1,2]
func TestIgnoreBlocksBelowFinalizedView(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(1, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// TestDoubleProposal tests that the DoubleProposal notification is emitted when two different
// proposals for the same view are added.
// receives [1,2] [2,3] [3,4] [4,5] [3,5']
// it should finalize block [2,3], and emits an DoubleProposal event with ([3,5'], [4,5])
func TestDoubleProposal(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.AddVersioned(3, 5, 0, 1)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[4].Block, blocks[3].Block).Once()

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
}

// TestConflictingQCs checks that adding 2 conflicting QCs should return model.ByzantineThresholdExceededError
// receives [1,2] [2,3] [2,3'] [3,4] [3',5]
// it should return fatal error, because conflicting blocks 3 and 3' both received enough votes for QC
func TestConflictingQCs(t *testing.T) {
	builder := forks.NewBlockBuilder()

	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.AddVersioned(2, 3, 0, 1) // make a conflicting proposal at view 3
	builder.Add(3, 4)                // creates a QC for 3
	builder.AddVersioned(3, 5, 1, 0) // creates a QC for 3'

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Return(nil)

	err = addBlocksToForks(forks, blocks)
	require.NotNil(t, err)
	assert.True(t, model.IsByzantineThresholdExceededError(err))
}

// TestConflictingFinalizedForks checks that finalizing 2 conflicting forks should return model.ByzantineThresholdExceededError
// receives [1,2] [2,3] [2,6] [3,4] [4,5] [6,7] [7,8]
// It should return fatal error, because 2 conflicting forks were finalized
func TestConflictingFinalizedForks(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5) // finalizes (2,3)
	builder.Add(2, 6)
	builder.Add(6, 7)
	builder.Add(7, 8) // finalizes (2,6) conflicts with (2,3)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Error(t, err)
	assert.True(t, model.IsByzantineThresholdExceededError(err))
}

// TestAddUnconnectedProposal checks that adding a proposal which does not connect to the
// latest finalized block returns an exception.
// receives [2,3]
// should return fatal error, because the proposal is invalid for addition to Forks
func TestAddUnconnectedProposal(t *testing.T) {
	unconnectedProposal := helper.MakeProposal(
		helper.WithBlock(helper.MakeBlock(
			helper.WithBlockView(3),
		)))

	forks, _ := newForks(t)

	err := forks.AddProposal(unconnectedProposal)
	require.Error(t, err)
	// adding a disconnected block is an internal error, should return generic error
	assert.False(t, model.IsByzantineThresholdExceededError(err))
}

// TestGetProposal tests that we can retrieve stored proposals.
// Attempting to retrieve nonexistent or pruned proposals should fail.
// receives [1,2] [2,3] [3,4], then [4,5]
// should finalize [1,2], then [2,3]
func TestGetProposal(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)
	blocksAddedFirst := blocks[:3]  // [1,2] [2,3] [3,4]
	blocksAddedSecond := blocks[3:] // [4,5]

	forks, _ := newForks(t)

	// should be unable to retrieve a block before it is added
	_, ok := forks.GetProposal(blocks[0].Block.BlockID)
	assert.False(t, ok)

	// add first blocks - should finalize [1,2]
	err = addBlocksToForks(forks, blocksAddedFirst)
	require.Nil(t, err)

	// should be able to retrieve all stored blocks
	for _, proposal := range blocksAddedFirst {
		got, ok := forks.GetProposal(proposal.Block.BlockID)
		assert.True(t, ok)
		assert.Equal(t, proposal, got)
	}

	// add second blocks - should finalize [2,3] and prune [1,2]
	err = addBlocksToForks(forks, blocksAddedSecond)
	require.Nil(t, err)

	// should be able to retrieve just added block
	got, ok := forks.GetProposal(blocksAddedSecond[0].Block.BlockID)
	assert.True(t, ok)
	assert.Equal(t, blocksAddedSecond[0], got)

	// should be unable to retrieve pruned block
	_, ok = forks.GetProposal(blocksAddedFirst[0].Block.BlockID)
	assert.False(t, ok)
}

// TestGetProposalsForView tests retrieving proposals for a view.
// receives [1,2] [2,4] [2,4']
func TestGetProposalsForView(t *testing.T) {

	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 4)
	builder.AddVersioned(2, 4, 0, 1)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[2].Block, blocks[1].Block).Once()

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	// 1 proposal at view 2
	proposals := forks.GetProposalsForView(2)
	assert.Len(t, proposals, 1)
	assert.Equal(t, blocks[0], proposals[0])

	// 2 proposals at view 4
	proposals = forks.GetProposalsForView(4)
	assert.Len(t, proposals, 2)
	assert.ElementsMatch(t, blocks[1:], proposals)

	// 0 proposals at view 3
	proposals = forks.GetProposalsForView(3)
	assert.Len(t, proposals, 0)
}

// TestNotification tests that notifier gets correct notifications when incorporating block as well as finalization events.
// receives [1,2] [2,3] [3,4]
// should finalize [1,2]
func TestNotification(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	notifier := &mocks.Consumer{}
	// 4 blocks including the genesis are incorporated
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(4)
	notifier.On("OnFinalizedBlock", blocks[0].Block).Return(nil).Once()
	finalizationCallback := mockmodule.NewFinalizer(t)
	finalizationCallback.On("MakeFinal", blocks[0].Block.BlockID).Return(nil).Once()

	forks, err := forks.New(builder.GenesisBlock(), finalizationCallback, notifier)
	require.NoError(t, err)

	err = addBlocksToForks(forks, blocks)
	require.NoError(t, err)
}

// TestNewestView tests that Forks tracks the newest block view seen in received blocks.
// receives [1,2] [2,3] [3,4]
func TestNewestView(t *testing.T) {
	builder := forks.NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	genesis := builder.GenesisBlock()

	// initially newest view should be genesis block view
	require.Equal(t, forks.NewestView(), genesis.Block.View)

	err = addBlocksToForks(forks, blocks)
	require.NoError(t, err)
	// after inserting new blocks, newest view should be greatest view of all added blocks
	require.Equal(t, forks.NewestView(), uint64(4))
}

// ========== internal functions ===============

func newForks(t *testing.T) (*forks.Forks, *mocks.Consumer) {
	notifier := mocks.NewConsumer(t)
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Maybe()
	notifier.On("OnFinalizedBlock", mock.Anything).Return(nil).Maybe()
	finalizationCallback := mockmodule.NewFinalizer(t)
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil).Maybe()

	genesisBQ := forks.NewBlockBuilder().GenesisBlock()

	forks, err := forks.New(genesisBQ, finalizationCallback, notifier)

	require.Nil(t, err)
	return forks, notifier
}

// addBlocksToForks adds all the given blocks to Forks, in order.
// If any errors occur, returns the first one.
func addBlocksToForks(forks *forks.Forks, proposals []*model.Proposal) error {
	for _, proposal := range proposals {
		err := forks.AddProposal(proposal)
		if err != nil {
			return fmt.Errorf("test case failed at adding proposal: %v: %w", proposal.Block.View, err)
		}
	}

	return nil
}

// requireLatestFinalizedBlock asserts that the latest finalized block has the given view and qc view.
func requireLatestFinalizedBlock(t *testing.T, forks *forks.Forks, qcView int, view int) {
	require.Equal(t, forks.FinalizedBlock().View, uint64(view), "finalized block has wrong view")
	require.Equal(t, forks.FinalizedBlock().QC.View, uint64(qcView), "finalized block has wrong qc")
}

// requireNoBlocksFinalized asserts that no blocks have been finalized (genesis is latest finalized block).
func requireNoBlocksFinalized(t *testing.T, f *forks.Forks) {
	genesis := forks.NewBlockBuilder().GenesisBlock()
	require.Equal(t, f.FinalizedBlock().View, genesis.Block.View)
	require.Equal(t, f.FinalizedBlock().View, genesis.CertifyingQC.View)
}
