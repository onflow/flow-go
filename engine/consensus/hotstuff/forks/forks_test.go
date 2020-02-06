package forks

import (
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

func TestForks_AddBlock(t *testing.T) {
	forks, notifier, root := initForks(t, 1)

	block02 := makeBlock(2, root.QC(), nil)
	notifier.On("OnBlockIncorporated", block02).Return().Once()
	err := forks.AddBlock(block02)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	notifier.AssertExpectations(t)
}

func initForks(t *testing.T, view uint64) (*Forks, *mockdist.Distributor, *types.QCBlock) {
	notifier := &mockdist.Distributor{}

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
