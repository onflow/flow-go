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
	viewState *ViewState
	builder   module.Builder

	// chainID is used for specifying the chainID field for new blocks
	chainID string
}

// NewBlockProducer creates a new BlockProducer
func NewBlockProducer(signer Signer, viewState *ViewState, builder module.Builder, chainID string) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		viewState: viewState,
		builder:   builder,
		chainID:   chainID,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(block *types.Block, qc *types.QuorumCertificate, view uint64) (*types.Proposal, error) {

	// create the block for the view
	block, err := bp.makeBlockForView(block, qc, view)
	if err != nil {
		return nil, fmt.Errorf("could not create block for view: %w", err)
	}

	// then sign the proposal
	proposal, err := bp.signer.Propose(block)
	if err != nil {
		return nil, fmt.Errorf("could not sign block proposal: %w", err)
	}

	return proposal, nil
}

// makeBlockForView gets the payload hash from mempool and build a block on top of the given qc for the given view.
func (bp *BlockProducer) makeBlockForView(parent *types.Block, qc *types.QuorumCertificate, view uint64) (*types.Block, error) {

	// define the block header build function
	// TODO: change this to return a *types.Block instead
	build := func(payloadHash flow.Identifier) (*flow.Header, error) {
		header := flow.Header{
			ChainID:       bp.chainID,
			ParentID:      parent.BlockID,
			ProposerID:    flow.ZeroID, // TODO: fill in our own ID here
			View:          view,
			Height:        parent.Height + 1,
			PayloadHash:   payloadHash,
			Timestamp:     time.Now().UTC(),
			ParentSigs:    qc.AggregatedSignature.Raw,
			ParentSigners: qc.AggregatedSignature.SignerIDs,
		}
		return &header, nil
	}

	// let the builder create the payload and store relevant stuff
	header, err := bp.builder.BuildOn(parent.BlockID, build)
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)

	}

	// turn the header into a block header proposal as known by hotstuff
	// TODO: probably need to populate a few more fields
	// TODO: use NewBlock to create block
	block := types.Block{
		BlockID:     header.ID(),
		View:        view,
		QC:          qc,
		PayloadHash: header.PayloadHash,
		Height:      header.Height,
		ChainID:     header.ChainID,
		Timestamp:   header.Timestamp,
	}

	return &block, nil
}
