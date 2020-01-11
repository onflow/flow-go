package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// BlockProducer needs to know when a block needs to be produced (OnForkChoiceGenerated),
// the current view (OnEnteringView), and the payload from the mempool
type DummyBlockProducer struct {
	signer Signer
	viewState ViewState
}

func (bp DummyBlockProducer) MakeBlockProposalWithQC(view uint64, qc *types.QuorumCertificate) *types.BlockProposal {
	block := types.NewBlock(view, qc, types.NewPayloadHash())
	unsignedBlockProposal := types.NewBlockProposal(block, types.NewConsensusPayload())
	blockProposal := bp.signer.SignBlockProposal(unsignedBlockProposal, bp.viewState.GetSelfIdxForView(view))

	return blockProposal
}


