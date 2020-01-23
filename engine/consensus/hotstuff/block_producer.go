package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// BlockProposalProducer is responsible for producing new block proposals
type BlockProposalProducer struct {
	signer    Signer
	viewState ViewState
	mempool   Mempool

	// chainID is used for specifying the chainID field for new blocks
	chainID string
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
func (bp *BlockProposalProducer) MakeBlockProposal(view uint64, qc *types.QuorumCertificate) (*types.BlockProposal, error) {
	block := bp.makeBlockForView(view, qc)

	unsignedBlockProposal := bp.propose(block)

	signedBlockProposal, err := bp.signBlockProposal(unsignedBlockProposal)

	return signedBlockProposal, err
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

// signBlockProposal takes a unsigned proposal, signes it and returns a signed block proposal
func (bp *BlockProposalProducer) signBlockProposal(proposal *types.UnsignedBlockProposal) (*types.BlockProposal, error) {
	selfIdx, err := bp.viewState.GetSelfIdxForBlockID(proposal.BlockMRH())
	if err != nil {
		return nil, fmt.Errorf("cannot sign block proposal, because %w", err)
	}
	sig := bp.signer.SignBlockProposal(proposal, selfIdx)

	blockProposal := types.NewBlockProposal(proposal.Block, proposal.ConsensusPayload, sig)
	return blockProposal, nil
}
