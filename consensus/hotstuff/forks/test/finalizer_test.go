package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	mockm "github.com/dapperlabs/flow-go/module/mock"
)

// denotion:
// A block is denoted as [<qc_number>, <block_view_number>].
// For example, [1,2] means: a block of view 2 has a QC for view 1.

// receives [1,2], [2,3], [3,4], [4,5],
// it should finalize [1,2], it should lock [2,3].
func TestLocked(t *testing.T) {
	builder := NewBlockBuilder()
	builder.Add(1, 2) // creates a block of view 2, with a QC of view 1
	builder.Add(2, 3)
	builder.Add(3, 4)
	builder.Add(4, 5)

	blocks, err := builder.Blocks()
	require.Nil(t, err)

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2) // check if the finalized block has view 1, and its QC is 2
	assertTheLockedBlock(t, fin, 2, 3)
	// ^^^ the reason it's not called "assertLockedBlock" is to match
	// its length with assertFinalizedBlock in order to align their arguments
}

// receives [1,2], [2,3], [3,4], [4,5], [4,6], [6,8]
// it should finalize [1,2], it should lock [3,4].
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 3, 4)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 4, 6)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 2, 3)
	assertTheLockedBlock(t, fin, 3, 4)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 5, 6)
	assertTheLockedBlock(t, fin, 6, 7)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 2, 3)
	assertTheLockedBlock(t, fin, 3, 4)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 3)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 3)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 3, 6)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 4, 6)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 3, 6)
	assertTheLockedBlock(t, fin, 6, 7)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 4, 6)
	assertTheLockedBlock(t, fin, 6, 7)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 3, 6)
	assertTheLockedBlock(t, fin, 6, 7)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 3)
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

	fin, notifier, _ := newFinalizer(t)
	notifier.On("OnDoubleProposeDetected", blocks[5], blocks[4]).Return(nil)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 2, 3)
	assertTheLockedBlock(t, fin, 3, 4)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 3)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 6)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 2, 3)
	assertTheLockedBlock(t, fin, 3, 4)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 1, 2)
	assertTheLockedBlock(t, fin, 2, 3)
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

	fin, notifier, _ := newFinalizer(t)
	notifier.On("OnDoubleProposeDetected", blocks[5], blocks[4]).Return(nil)

	err = addBlocksToFinalizer(fin, blocks)
	require.Nil(t, err)

	assertFinalizedBlock(t, fin, 2, 3)
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

	fin, notifier, _ := newFinalizer(t)
	notifier.On("OnDoubleProposeDetected", blocks[3], blocks[2]).Return(nil)

	err = addBlocksToFinalizer(fin, blocks)
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

	fin, _, _ := newFinalizer(t)

	err = addBlocksToFinalizer(fin, blocks)
	assert.Error(t, err)
}

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
	notifier.On("OnFinalizedBlock", blocks[0]).Return(nil).Once()
	finalizationCallback := &mockm.Finalizer{}
	finalizationCallback.On("MakeFinal", blocks[0].BlockID).Return(nil).Once()
	finalizationCallback.On("MakeValid", mock.Anything).Return(nil)

	genesisBQ := makeGenesis()

	fin, err := finalizer.New(genesisBQ, finalizationCallback, notifier)
	require.NoError(t, err)

	err = addBlocksToFinalizer(fin, blocks)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
	finalizationCallback.AssertExpectations(t)
}

// ========== internal functions ===============

func newFinalizer(t *testing.T) (forks.Finalizer, *mocks.Consumer, *mockm.Finalizer) {
	notifier := &mocks.Consumer{}
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil)
	notifier.On("OnFinalizedBlock", mock.Anything).Return(nil)
	finalizationCallback := &mockm.Finalizer{}
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil)
	finalizationCallback.On("MakeValid", mock.Anything).Return(nil)

	genesisBQ := makeGenesis()

	fin, err := finalizer.New(genesisBQ, finalizationCallback, notifier)

	require.Nil(t, err)
	return fin, notifier, finalizationCallback
}

func addBlocksToFinalizer(fin forks.Finalizer, blocks []*model.Block) error {
	for _, block := range blocks {
		err := fin.AddBlock(block)
		if err != nil {
			return fmt.Errorf("test case failed at adding block: %v: %w", block.View, err)
		}
	}

	return nil
}

// check the view and QC's view of the locked block for the finalizer
func assertTheLockedBlock(t *testing.T, fin forks.Finalizer, qc int, view int) {
	assert.Equal(t, fin.LockedBlock().View, uint64(view), "locked block has wrong view")
	assert.Equal(t, fin.LockedBlock().QC.View, uint64(qc), "locked block has wrong qc")
}

// check the view and QC's view of the finalized block for the finalizer
func assertFinalizedBlock(t *testing.T, fin forks.Finalizer, qc int, view int) {
	assert.Equal(t, fin.FinalizedBlock().View, uint64(view), "finalized block has wrong view")
	assert.Equal(t, fin.FinalizedBlock().QC.View, uint64(qc), "fianlized block has wrong qc")
}
