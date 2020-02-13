package forks

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/forkchoice"
	mocknotifier "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	mockfinalizer "github.com/dapperlabs/flow-go/module/mock"
	"github.com/stretchr/testify/assert"
)

// TestForks_ImplementsInterface tests that forks.Forks implements hotstuff.Forks
// (compile-time test)
func TestForks_ImplementsInterface(t *testing.T) {
	var _ hotstuff.Forks = &Forks{}
}

// TestForks_Initialization tests that Forks correctly reports trusted Root
func TestForks_Initialization(t *testing.T) {
	forks, _, _, root := initForks(t, 1)

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
	forks, _, notifier, root := initForks(t, 1)

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
	forks, finCallback, notifier, root := initForks(t, 1) // includes genesis block (v1)

	block2 := makeBlock(2, root.QC(), nil)
	notifier.On("OnBlockIncorporated", block2).Return().Once()
	addBlock2Forks(t, block2, forks)
	notifier.AssertExpectations(t)

	block3 := makeBlock(3, qc(block2.View(), block2.BlockID()), nil)
	notifier.On("OnBlockIncorporated", block3).Return().Once()
	notifier.On("OnQcIncorporated", block3.QC()).Return().Once()
	addBlock2Forks(t, block3, forks)
	notifier.AssertExpectations(t)

	// creates direct 3-chain on genesis block (1), which is already finalized  
	block4 := makeBlock(4, qc(block3.View(), block3.BlockID()), nil)
	notifier.On("OnBlockIncorporated", block4).Return().Once()
	notifier.On("OnQcIncorporated", block4.QC()).Return().Once()
	addBlock2Forks(t, block4, forks)
	notifier.AssertExpectations(t)

	// creates direct 3-chain on block (2) => finalize (2)  
	block5 := makeBlock(5, qc(block4.View(), block4.BlockID()), nil)
	notifier.On("OnBlockIncorporated", block5).Return().Once()
	notifier.On("OnQcIncorporated", block5.QC()).Return().Once()
	notifier.On("OnFinalizedBlock", block2).Return().Once()
	finCallback.On("MakeFinal", block2.BlockID()).Return(nil).Once()
	addBlock2Forks(t, block5, forks)
	notifier.AssertExpectations(t)
	finCallback.AssertExpectations(t)

	// creates direct 3-chain on block (3) => finalize (3)  
	block6 := makeBlock(6, qc(block5.View(), block5.BlockID()), nil)
	notifier.On("OnBlockIncorporated", block6).Return().Once()
	notifier.On("OnQcIncorporated", block6.QC()).Return().Once()
	notifier.On("OnFinalizedBlock", block3).Return().Once()
	finCallback.On("MakeFinal", block3.BlockID()).Return(nil).Once()
	addBlock2Forks(t, block6, forks)
	notifier.AssertExpectations(t)
	finCallback.AssertExpectations(t)
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

func initForks(t *testing.T, view uint64) (*Forks, *mockfinalizer.Finalizer, *mocknotifier.Consumer, *types.QCBlock) {
	notifier := &mocknotifier.Consumer{}
	finalizationCallback := &mockfinalizer.Finalizer{}

	// construct Finalizer
	root := makeRootBlock(t, view)
	notifier.On("OnBlockIncorporated", root.Block()).Return().Once()
	fnlzr, err := finalizer.New(root, finalizationCallback, notifier)
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
	return New(fnlzr, fc), finalizationCallback, notifier, root
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

func makeChildBlock(blockView uint64, payloadHash []byte, parent *types.BlockProposal) *types.BlockProposal {
	qcForParent := qc(parent.View(), parent.BlockID())
	id := computeID(blockView, qcForParent, payloadHash)
	return &types.BlockProposal{
		Block: types.NewBlock(id, blockView, qcForParent, payloadHash, 0, ""),
	}
}

func makeBlock(blockView uint64, blockQc *types.QuorumCertificate, payloadHash []byte) *types.BlockProposal {
	if blockQc == nil {
		blockQc = qc(0, flow.Identifier{})
	}
	id := computeID(blockView, blockQc, payloadHash)
	return &types.BlockProposal{
		Block: types.NewBlock(id, blockView, blockQc, payloadHash, 0, ""),
	}
}

func string2Identifyer(s string) flow.Identifier {
	var identifier flow.Identifier
	copy(identifier[:], []byte(s))
	return identifier
}

// computeID is an INCOMPLETE STUB needed so we can test Forks.
// When computing the Block's ID, this implementation only considers
// the fields used by Forks.
// TODO need full implementation
func computeID(view uint64, qc *types.QuorumCertificate, payloadHash []byte) flow.Identifier {
	id := make([]byte, 0)

	viewBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(viewBytes, view)
	id = append(id, viewBytes...)

	qcView := make([]byte, 8)
	binary.BigEndian.PutUint64(qcView, qc.View)
	id = append(id, qcView...)
	id = append(id, qc.BlockID[:]...)

	id = append(id, payloadHash...)

	hasher := crypto.NewSHA3_256()
	hash := hasher.ComputeHash(id)

	var identifier flow.Identifier
	copy(identifier[:], hash)
	return identifier
}
