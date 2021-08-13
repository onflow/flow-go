package blockproducer

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	signer    hotstuff.SignerVerifier
	committee hotstuff.Committee
	builder   module.Builder
}

// New creates a new BlockProducer which wraps the chain compliance layer block builder
// to provide hotstuff with block proposals.
func New(signer hotstuff.SignerVerifier, committee hotstuff.Committee, builder module.Builder) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		committee: committee,
		builder:   builder,
	}
	return bp, nil
}

// MakeBlockProposal will build a proposal for the given view with the given QC
func (bp *BlockProducer) MakeBlockProposal(qc *flow.QuorumCertificate, view uint64) (*model.Proposal, error) {
	// the custom functions allows us to set some custom fields on the block;
	// in hotstuff, we use this for view number and signature-related fields
	setHotstuffFields := func(header *flow.Header) error {
		header.View = view
		header.ParentVoterIDs = qc.SignerIDs
		header.ParentVoterSigData = qc.SigData
		header.ProposerID = bp.committee.Self()

		// turn the header into a block header proposal as known by hotstuff
		block := model.Block{
			BlockID:     header.ID(),
			View:        view,
			ProposerID:  header.ProposerID,
			QC:          qc,
			PayloadHash: header.PayloadHash,
			Timestamp:   header.Timestamp,
		}

		// then sign the proposal
		proposal, err := bp.signer.CreateProposal(&block)
		if err != nil {
			return fmt.Errorf("could not sign block proposal: %w", err)
		}

		header.ProposerSigData = proposal.SigData
		return nil
	}

	// retrieve a fully built block header from the builder
	header, err := bp.builder.BuildOn(qc.BlockID, setHotstuffFields)
	if err != nil {
		return nil, fmt.Errorf("could not build block proposal on top of %v: %w", qc.BlockID, err)
	}

	// turn the signed flow header into a proposal
	proposal := model.ProposalFromFlow(header, qc.View)

	return proposal, nil
}
