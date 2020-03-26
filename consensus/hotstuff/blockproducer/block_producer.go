package blockproducer

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	signer    hotstuff.Signer
	viewState hotstuff.ViewState
	builder   module.Builder
}

// New creates a new BlockProducer which wraps the chain compliance layer block builder
// to provide hotstuff with block proposals.
func New(signer hotstuff.Signer, viewState hotstuff.ViewState, builder module.Builder) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		viewState: viewState,
		builder:   builder,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(qc *model.QuorumCertificate, view uint64) (*model.Proposal, error) {

	// create the block for the view
	block, err := bp.makeBlockForView(qc, view)
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
func (bp *BlockProducer) makeBlockForView(qc *model.QuorumCertificate, view uint64) (*model.Block, error) {

	// the custom functions allows us to set some custom fields on the block;
	// in hotstuff, we use this for view number and signature-related fields
	setHotstuffFields := func(header *flow.Header) {
		header.View = view
		header.ProposerID = bp.viewState.Self()
		header.ParentStakingSigs = qc.AggregatedSignature.StakingSignatures
		header.ParentRandomBeaconSig = qc.AggregatedSignature.RandomBeaconSignature
		header.ParentSigners = qc.AggregatedSignature.SignerIDs
	}

	// retrieve a fully built block header from the builder
	header, err := bp.builder.BuildOn(qc.BlockID, setHotstuffFields)
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)
	}

	// turn the header into a block header proposal as known by hotstuff
	block := model.Block{
		BlockID:     header.ID(),
		View:        view,
		ProposerID:  header.ProposerID,
		QC:          qc,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}

	return &block, nil
}
