package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// BlockProposalProducer is responsible for producing new block proposals
type BlockProposalProducer struct {
	signer    Signer
	viewState ViewState
	mempool   Mempool
	chainID   string
}

func NewBlockProposalProducer(signer Signer, viewState ViewState, mempool Mempool, chainID string) (*BlockProposalProducer, error) {
	bp := &BlockProposalProducer{
		signer:    signer,
		viewState: viewState,
		mempool:   mempool,
		chainID:   chainID,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProposalProducer) MakeBlockProposal(view uint64, qc *types.QuorumCertificate) *types.BlockProposal {
	block := bp.makeBlockForView(view, qc)

	unsignedBlockProposal := bp.propose(block)

	sig := bp.signer.SignBlockProposal(unsignedBlockProposal, bp.viewState.GetSelfIdxForView(view))

	blockProposal := types.NewBlockProposal(unsignedBlockProposal.Block, unsignedBlockProposal.ConsensusPayload, sig)

	return blockProposal
}

// makeBlockForView gets the payload hash from mempool and build a block on top of the given qc for the given view.
func (bp *BlockProposalProducer) makeBlockForView(view uint64, qc *types.QuorumCertificate) *types.Block {
	payloadHash := bp.mempool.NewPayloadHash()

	// TODO: current use view as height
	height := view

	block := types.NewBlock(view, qc, payloadHash, height, bp.chainID)
	return block
}

// propose get the consensus payload from mempool and make unsigned block proposal with the given block
func (bp *BlockProposalProducer) propose(block *types.Block) *types.UnsignedBlockProposal {
	consensusPayload := bp.mempool.NewConsensusPayload()
	unsignedBlockProposal := types.NewUnsignedBlockProposal(block, consensusPayload)
	return unsignedBlockProposal
}
