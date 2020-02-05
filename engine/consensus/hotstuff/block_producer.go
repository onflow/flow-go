package hotstuff

import (
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	signer    Signer
	viewState ViewState
	builder   module.Builder

	// chainID is used for specifying the chainID field for new blocks
	chainID string
}

func NewBlockProducer(signer Signer, viewState ViewState, mempool Mempool, chainID string) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		viewState: viewState,
		chainID:   chainID,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(view uint64, qcblock *types.QCBlock) (*types.BlockProposal, error) {

	// create the block for the view
	block, err := bp.makeBlockForView(view, qcblock)
	if err != nil {
		return nil, fmt.Errorf("could not create block for view: %w", err)
	}

	// then sign the proposal
	signedBlockProposal, err := bp.signBlockProposal(block)
	if err != nil {
		return nil, fmt.Errorf("could not sign block proposal: %w", err)
	}

	return signedBlockProposal, nil
}

// makeBlockForView gets the payload hash from mempool and build a block on top of the given qc for the given view.
func (bp *BlockProducer) makeBlockForView(view uint64, qcblock *types.QCBlock) (*types.Block, error) {

	// define the block header build function
	build := func(payloadHash flow.Identifier) (*flow.Header, error) {
		header := flow.Header{
			ChainID:      bp.chainID,
			Number:       view,
			Height:       qcblock.Block.Height() + 1,
			Timestamp:    time.Now().UTC(),
			ParentID:     qcblock.Block.BlockID(),
			ParentNumber: qcblock.Block.View(),
			PayloadHash:  payloadHash,
			ProposerID:   flow.ZeroID, // TODO: fill in our own ID here
		}
		return &header, nil
	}

	// let the builder create the payload and store relevant stuff
	header, err := bp.builder.BuildOn(qcblock.Block.BlockID(), build)
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)
	}

	// turn the header into a block header proposal as known by hotstuff
	// TODO: probably need to populate a few more fields
	block := types.NewBlock(header.ID(), header.Number, qcblock.QC, header.PayloadHash[:], header.Number, header.ChainID)

	return block, nil
}

// signBlockProposal takes a unsigned proposal, signes it and returns a signed block proposal
func (bp *BlockProducer) signBlockProposal(proposal *types.Block) (*types.BlockProposal, error) {
	// get my identity
	myIdentity, err := bp.viewState.GetSelfIdentityForBlockID(proposal.BlockID)
	if err != nil {
		return nil, err
	}

	// convert the proposal into a vote
	unsignedVote := proposal.ToVote()

	// signing the proposal is equivalent of signing the vote
	sig := bp.signer.SignVote(unsignedVote, myIdentity.PubKey)

	blockProposal := types.NewBlockProposal(proposal, sig)
	return blockProposal, nil
}
