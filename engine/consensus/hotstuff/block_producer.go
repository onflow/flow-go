package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	signer    signature.Signer
	viewState ViewState
	builder   module.Builder

	// chainID is used for specifying the chainID field for new blocks
	chainID string
}

func NewBlockProducer(signer signature.Signer, viewState ViewState, builder module.Builder, chainID string) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		viewState: viewState,
		builder:   builder,
		chainID:   chainID,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(view uint64, qcblock *types.QCBlock) (*types.BlockProposal, error) {
	block, err := bp.makeBlockForView(view, qcblock)
	if err != nil {
		return nil, err
	}

	signedBlockProposal, err := bp.signBlockProposal(block)
	if err != nil {
		return nil, err
	}

	return signedBlockProposal, nil
}

// makeBlockForView builds a new block on top of the given QC for the given
// view using the builder module to generate a payload.
func (bp *BlockProducer) makeBlockForView(view uint64, qcblock *types.QCBlock) (*types.Block, error) {
	// TODO block should use flow.Identifier, bubble up error
	parentID := qcblock.Block.BlockID()
	payloadHash, err := bp.builder.BuildOn(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not generate payload: %w", err)
	}

	// new block's height = parent.height + 1
	height := qcblock.Block.Height() + 1

	block := types.NewBlock(view, qcblock.QC, payloadHash[:], height, bp.chainID)
	return block, nil
}

// signBlockProposal takes a unsigned proposal, signes it and returns a signed block proposal
func (bp *BlockProducer) signBlockProposal(proposal *types.Block) (*types.BlockProposal, error) {
	// get my public key and signer index
	myIndexedPubKey, err := bp.viewState.GetSelfIndexPubKeyForBlockID(proposal.ID())
	if err != nil {
		return nil, err
	}

	// convert the proposal into a vote
	unsignedVote := proposal.ToVote()

	// signing the proposal is equivalent of signing the vote
	sig := bp.signer.SignVote(unsignedVote, myIndexedPubKey.PubKey)

	blockProposal := types.NewBlockProposal(proposal, sig)
	return blockProposal, nil
}
