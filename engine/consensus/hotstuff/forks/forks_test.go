package forks

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/forkchoice"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
)

// TestForks_ImplementsInterface tests that forks.Forks implements hotstuff.Forks
// (compile-time test)
func TestForks_ImplementsInterface(t *testing.T) {
	var _ hotstuff.Forks = &Forks{}
}

// TestForks_Initialization tests that Forks correctly reports trusted Root
func TestForks_Initialization(t *testing.T) {
	forks, _, root := initForks(t, 1)

	assert.Equal(t, forks.FinalizedView(), uint64(1))
	assert.Equal(t, forks.FinalizedBlock(), root.Block())

	assert.Equal(t, forks.GetBlocksForView(0), []*types.BlockProposal{})
	assert.Equal(t, forks.GetBlocksForView(1), []*types.BlockProposal{root.Block()})
	assert.Equal(t, forks.GetBlocksForView(2), []*types.BlockProposal{})

	b, found := forks.GetBlock(root.BlockID())
	assert.True(t, found, "Missing trusted Root ")
	assert.Equal(t, root.Block(), b)
}

// TestForks_AddBlock verifies that Block can be added
func TestForks_AddBlock(t *testing.T) {
	forks, notifier, root := initForks(t, 1)

	block02 := makeBlock(2, root.QC(), nil)
	notifier.On("OnBlockIncorporated", block02).Return().Once()
	err := forks.AddBlock(block02)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	notifier.AssertExpectations(t)

	assert.Equal(t, forks.GetBlocksForView(2), []*types.BlockProposal{block02})
	b, found := forks.GetBlock(block02.BlockID())
	assert.True(t, found)
	assert.Equal(t, block02, b)
}

// TestForks_3ChainFinalization tests happy-path direct 3-chain finalization
func TestForks_3ChainFinalization(t *testing.T) {
	forks, notifier, root := initForks(t, 1)

	targetBlock := makeBlock(2, root.QC(), nil)
	notifier.On("OnBlockIncorporated", targetBlock).Return().Once()
	addBlock2Forks(t, targetBlock, forks)
	notifier.AssertExpectations(t)

	targetBlockPlus1 := makeBlock(3, qc(targetBlock.View(), targetBlock.BlockID()), nil)
	notifier.On("OnBlockIncorporated", targetBlockPlus1).Return().Once()
	notifier.On("OnQcIncorporated", targetBlockPlus1.QC()).Return().Once()
	addBlock2Forks(t, targetBlockPlus1, forks)
	notifier.AssertExpectations(t)

	targetBlockPlus2 := makeBlock(4, qc(targetBlockPlus1.View(), targetBlockPlus1.BlockID()), nil)
	notifier.On("OnBlockIncorporated", targetBlockPlus2).Return().Once()
	notifier.On("OnQcIncorporated", targetBlockPlus2.QC()).Return().Once()
	addBlock2Forks(t, targetBlockPlus2, forks)
	notifier.AssertExpectations(t)

	targetBlockPlus3 := makeBlock(5, qc(targetBlockPlus2.View(), targetBlockPlus2.BlockID()), nil)
	notifier.On("OnBlockIncorporated", targetBlockPlus3).Return().Once()
	notifier.On("OnQcIncorporated", targetBlockPlus3.QC()).Return().Once()
	notifier.On("OnFinalizedBlock", targetBlock).Return().Once()
	addBlock2Forks(t, targetBlockPlus3, forks)
	notifier.AssertExpectations(t)

	targetBlockPlus4 := makeBlock(6, qc(targetBlockPlus3.View(), targetBlockPlus3.BlockID()), nil)
	notifier.On("OnBlockIncorporated", targetBlockPlus4).Return().Once()
	notifier.On("OnQcIncorporated", targetBlockPlus4.QC()).Return().Once()
	notifier.On("OnFinalizedBlock", targetBlockPlus1).Return().Once()
	addBlock2Forks(t, targetBlockPlus4, forks)
	notifier.AssertExpectations(t)
}

func addBlock2Forks(t *testing.T, block *types.BlockProposal, forks hotstuff.Forks) {
	err := forks.AddBlock(block)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	verifyStored(t, block, forks)
}

// verifyStored verifies that block is stored in forks
func verifyStored(t *testing.T, block *types.BlockProposal, forks hotstuff.Forks) {
	b, found := forks.GetBlock(block.BlockID())
	assert.True(t, found)
	assert.Equal(t, block, b)

	found = false
	siblings := forks.GetBlocksForView(block.View())
	assert.True(t, len(siblings) > 0)
	for _, b := range forks.GetBlocksForView(block.View()) {
		if b != block {
			continue
		}
		if found { // we already found block in slice, i.e. this is a duplicate
			assert.Fail(t, fmt.Sprintf("Duplicate block: %v", block.BlockID()))
		}
		found = true
	}
	assert.True(t, found, fmt.Sprintf("Did not find block: %v", block.BlockID()))
}

func initForks(t *testing.T, view uint64) (*Forks, *mockdist.Consumer, *types.QCBlock) {
	notifier := &mockdist.Consumer{}

	// construct Finalizer
	root := makeRootBlock(t, view)
	notifier.On("OnBlockIncorporated", root.Block()).Return().Once()
	fnlzr, err := finalizer.New(root, notifier)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	// construct ForkChoice
	notifier.On("OnQcIncorporated", root.QC()).Return().Once()
	fc, err := forkchoice.NewNewestForkChoice(fnlzr, notifier)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	notifier.AssertExpectations(t)
	return New(fnlzr, fc), notifier, root
}

func makeRootBlock(t *testing.T, view uint64) *types.QCBlock {
	// construct Finalizer with Genesis Block
	genesisBlock := makeBlock(view, nil, nil)
	genesisQC := qc(view, genesisBlock.BlockID())
	root, err := types.NewQcBlock(genesisQC, genesisBlock)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	return root
}

func qc(view uint64, id flow.Identifier) *types.QuorumCertificate {
	return &types.QuorumCertificate{View: view, BlockID: id}
}

func makeChildBlock(blockView uint64, parent *types.BlockProposal, payloadHash []byte) *types.BlockProposal {
	return &types.BlockProposal{
		Block: types.NewBlock(blockView, qc(parent.View(), parent.BlockID()), payloadHash, 0, ""),
	}
}

func makeBlock(blockView uint64, blockQc *types.QuorumCertificate, payloadHash []byte) *types.BlockProposal {
	if blockQc == nil {
		blockQc = qc(0, flow.Identifier{})
	}
	return &types.BlockProposal{
		Block: types.NewBlock(blockView, blockQc, payloadHash, 0, ""),
	}
}

func string2Identifyer(s string) flow.Identifier {
	var identifier flow.Identifier
	copy(identifier[:], []byte(s))
	return identifier
}
