package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	signer    Signer
	viewState *ViewState
	mempool   Mempool

	// chainID is used for specifying the chainID field for new blocks
	chainID string
}

func NewBlockProducer(signer Signer, viewState *ViewState, mempool Mempool, chainID string) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		viewState: viewState,
		mempool:   mempool,
		chainID:   chainID,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(view uint64, qcblock *types.QCBlock) *types.BlockProposal {
	block := bp.makeBlockForView(view, qcblock)

	signedBlockProposal := bp.signBlockProposal(block)

	return signedBlockProposal
}

// makeBlockForView gets the payload hash from mempool and build a block on top of the given qc for the given view.
func (bp *BlockProducer) makeBlockForView(view uint64, qcblock *types.QCBlock) *types.Block {
	payloadHash := bp.mempool.NewPayloadHash()

	// new block's height = parent.height + 1
	height := qcblock.Block.Height() + 1

	block := types.NewBlock(view, qcblock.QC, payloadHash, height, bp.chainID)
	return block
}

// signBlockProposal takes a unsigned proposal, signes it and returns a signed block proposal
func (bp *BlockProducer) signBlockProposal(proposal *types.Block) *types.BlockProposal {
	identity := bp.viewState.GetSelfIdxForBlockID(proposal.BlockID())
	sig := bp.signer.SignBlockProposal(proposal, identity)

	blockProposal := types.NewBlockProposal(proposal.Block, sig)
	return blockProposal
}
