package forks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	mockmodule "github.com/onflow/flow-go/module/mock"
)

// NOTATION:
// A block is denoted as [<qc_number>, <block_view_number>].
// For example, [1,2] means: a block of view 2 has a QC for view 1.

// receives [1,2], [2,3]
// it should not finalize any block
func TestFinalize_1Chain(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireNoBlocksFinalized(t, forks)
}

// receives [1,2], [2,3], [3,4]
// it should finalize [1,2]
func TestFinalize_Direct2Chain(t *testing.T) {
	builder := NewBlockBuilder()
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

// receives [1,2], [2,3], [3,5]
// it should finalize [1,2]
func TestFinalize_DirectIndirect2Chain(t *testing.T) {
	builder := NewBlockBuilder()
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

// receives [1,2], [2,4], [4,5]
// it should not finalize any blocks
func TestFinalize_IndirectDirect2Chain(t *testing.T) {
	builder := NewBlockBuilder()
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

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,8]
// it should finalize [1,2]
func TestLocked2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 6)
	builder.Add(6, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,8], [8,10]
// it should finalize [1,2], it should lock [4,6].
func TestLocked3(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 6)
	builder.Add(6, 8)
	builder.Add(8, 10)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [5,6]
// it should finalize [2,3], it should lock [3,4]
func TestFinalizedDirect3builder(t *testing.T) {
	builder := NewBlockBuilder()
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

	requireLatestFinalizedBlock(t, forks, 2, 3)
}

// receives [1,2], [2,3], [3,4], [4,5], [5,6], [6,7], [7,8], [8, 9]
// it should finalize [5,6], it should lock [6,7]
func TestFinalizedDirect3builder2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)
	builder.Add(8, 9)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 5, 6)
}

// receives [1,2], [2,3], [3,4], [4,5], [5,7],
// it should finalize [2,3], it should lock [3,4]
func TestFinalizedDirect2builderPlus1builder(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 7)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6],
// it should finalize [1,2], it should lock [2,3]
func TestUnfinalized(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,7],
// it should finalize [1,2], it should lock [2,3]
func TestUnfinalized2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 7)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// Tolerable Forks that extend from locked block (1: might change locked block, 2: not change locked block)
// receives [1,2], [2,3], [3,4], [4,5], [3,6], [6,7], [7,8]
// it should finalize [1,2], it should lock [3,6]
func TestTolerableForksExtendsFromLockedBlock(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(3, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,7], [7,8]
// it should finalize [1,2], it should lock [4,6]
func TestTolerableForksExtendsFromLockedBlock2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [3,6], [6,7], [7,8], [8,9]
// it should finalize [3,6], it should lock [6,7]
func TestTolerableForksExtendsFromLockedBlock3(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(3, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)
	builder.Add(8, 9)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 3, 6)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,7], [7,8], [8,9]
// it should finalize [4,6], it should lock [6,7]
func TestTolerableForksExtendsFromLockedBlock4(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(4, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)
	builder.Add(8, 9)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 4, 6)
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,7], [7,8], [8,10]
// it should finalize [3,6], it should lock [6,7]
func TestTolerableForksExtendsFromLockedBlock5(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(3, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)
	builder.Add(8, 10)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 3, 6)
}

// receives [1,2], [2,3], [3,4], [4,5], [2,6]
// it should finalize [1,2], it should lock [2,3]
func TestTolerableForksNotExtendsFromLockedBlock(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(2, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [2,6], [5,6]
// it should finalize [2,3], it should lock [3,4], because [2,6] is replaced by [5,6]
func TestTolerableForksNotExtendsFromLockedBlock2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(2, 6)
	builder.Add(5, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[5].Block, blocks[4].Block).Return(nil)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
	notifier.AssertExpectations(t)
}

// receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7]
// it should finalize [1,2], it should lock [2,3]
func TestTolerableForksNotExtendsFromLockedBlock3(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(2, 6)
	builder.Add(6, 7)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7],[7,8]
// it should finalize [1,2], it should lock [2,6]
func TestTolerableForksNotExtendsFromLockedBlock4(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(2, 6)
	builder.Add(6, 7)
	builder.Add(7, 8)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [2,3], [3,4], [3,4], [4,5], [4,5], [5,6], [5,6]
// it should finalize [2,3], it should lock [3,4]
func TestDuplication(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 6)
	builder.Add(4, 5)
	builder.Add(5, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
}

// receives [1,2], [2,3], [3,4], [4,5], [1,6]
// it should finalize [1,2], it should lock [2,3]
func TestIgnoreBlocksBelowFinalizedView(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(1, 6)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 1, 2)
}

// receives [1,2], [2,3], [3,4], [4,5], [3,6], [5,6'].
// it should finalize block [2,3], and emits an DoubleProposal event with ([3,6], [5,6'])
func TestDoubleProposal(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(3, 6)
	builder.AddVersioned(5, 6, 0, 1)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[5].Block, blocks[4].Block).Return(nil)

	err = addBlocksToForks(forks, blocks)
	require.Nil(t, err)

	requireLatestFinalizedBlock(t, forks, 2, 3)
	notifier.AssertExpectations(t)
}

// receives [1,2], [2,3], [3,4], [3,4'], [4,5], [4',6].
// it should return fatal error, because conflicting blocks 4 and 4'
// both received enough votes for QC
func TestUntolerableForks(t *testing.T) {
	builder := NewBlockBuilder()

	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.AddVersioned(3, 4, 0, 1) // make a special view 4
	builder.Add(4, 5)
	builder.AddVersioned(4, 6, 1, 0) // make a special view 6 extends from special view 4

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, notifier := newForks(t)
	notifier.On("OnDoubleProposeDetected", blocks[3].Block, blocks[2].Block).Return(nil)

	err = addBlocksToForks(forks, blocks)
	require.NotNil(t, err)
	notifier.AssertExpectations(t)
}

// receives [1,2], [2,3], [2,7], [3,4], [4,5], [5,6], [7,8], [8,9], [9,10]
// It should return fatal error, because a fork below locked block got finalized
func TestUntolerableForks2(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)
	builder.Add(5, 6) // this finalizes (2,3)
	builder.Add(2, 7)
	builder.Add(7, 8)
	builder.Add(8, 9)
	builder.Add(9, 10) // this finalizes (2,7), which is a conflicting fork with (2,3)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	err = addBlocksToForks(forks, blocks)
	require.Error(t, err)
}

// TestNotification tests that notifier gets correct notifications when incorporating block as well as finalization events.
func TestNotification(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	notifier := &mocks.Consumer{}
	// 5 blocks including the genesis are incorporated
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil).Times(5)
	notifier.On("OnFinalizedBlock", blocks[0].Block).Return(nil).Once()
	finalizationCallback := &mockmodule.Finalizer{}
	finalizationCallback.On("MakeFinal", blocks[0].Block.BlockID).Return(nil).Once()
	finalizationCallback.On("MakeValid", mock.Anything).Return(nil)

	genesisBQ := makeGenesis()

	forks, err := New(genesisBQ, finalizationCallback, notifier)
	require.NoError(t, err)

	err = addBlocksToForks(forks, blocks)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
	finalizationCallback.AssertExpectations(t)
}

// TestNewestView tests that Forks tracks the newest block view seen in received blocks.
func TestNewestView(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2)
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	forks, _ := newForks(t)

	genesis := makeGenesis()

	require.Equal(t, forks.NewestView(), genesis.Block.View)

	err = addBlocksToForks(forks, blocks)
	require.NoError(t, err)
	require.Equal(t, forks.NewestView(), blocks[len(blocks)-1].Block.View)
}

// ========== internal functions ===============

func newForks(t *testing.T) (*Forks, *mocks.Consumer) {
	notifier := &mocks.Consumer{}
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil)
	notifier.On("OnFinalizedBlock", mock.Anything).Return(nil)
	finalizationCallback := &mockmodule.Finalizer{}
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil)
	finalizationCallback.On("MakeValid", mock.Anything).Return(nil)

	genesisBQ := makeGenesis()

	forks, err := New(genesisBQ, finalizationCallback, notifier)

	require.Nil(t, err)
	return forks, notifier
}

func addBlocksToForks(forks *Forks, proposals []*model.Proposal) error {
	for _, proposal := range proposals {
		err := forks.AddProposal(proposal)
		if err != nil {
			return fmt.Errorf("test case failed at adding proposal: %v: %w", proposal.Block.View, err)
		}
	}

	return nil
}

// requireLatestFinalizedBlock asserts that the latest finalized block has the given view and qc view.
func requireLatestFinalizedBlock(t *testing.T, forks *Forks, qcView int, view int) {
	require.Equal(t, forks.FinalizedBlock().View, uint64(view), "finalized block has wrong view")
	require.Equal(t, forks.FinalizedBlock().QC.View, uint64(qcView), "finalized block has wrong qc")
}

// requireNoBlocksFinalized asserts that no blocks have been finalized (genesis is latest finalized block).
func requireNoBlocksFinalized(t *testing.T, forks *Forks) {
	genesis := makeGenesis()
	require.Equal(t, forks.FinalizedBlock().View, genesis.Block.View)
	require.Equal(t, forks.FinalizedBlock().View, genesis.QC.View)
}
