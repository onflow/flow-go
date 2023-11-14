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
	signer    hotstuff.Signer
	committee hotstuff.Replicas
	builder   module.Builder
}

var _ hotstuff.BlockProducer = (*BlockProducer)(nil)

// New creates a new BlockProducer which wraps the chain compliance layer block builder
// to provide hotstuff with block proposals.
// No errors are expected during normal operation.
func New(signer hotstuff.Signer, committee hotstuff.Replicas, builder module.Builder) (*BlockProducer, error) {
	bp := &BlockProducer{
		signer:    signer,
		committee: committee,
		builder:   builder,
	}
	return bp, nil
}

// MakeBlockProposal builds a new HotStuff block proposal using the given view,
// the given quorum certificate for its parent and [optionally] a timeout certificate for last view(could be nil).
// No errors are expected during normal operation.
func (bp *BlockProducer) MakeBlockProposal(view uint64, qc *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*flow.Header, error) {
	// the custom functions allows us to set some custom fields on the block;
	// in hotstuff, we use this for view number and signature-related fields
	setHotstuffFields := func(header *flow.Header) error {
		header.View = view
		header.ParentView = qc.View
		header.ParentVoterIndices = qc.SignerIndices
		header.ParentVoterSigData = qc.SigData
		header.ProposerID = bp.committee.Self()
		header.LastViewTC = lastViewTC
		return nil
	}

	// TODO: We should utilize the `EventHandler`'s `SafetyRules` to generate the block signature instead of using an independent signing logic: https://github.com/dapperlabs/flow-go/issues/6892
	signProposal := func(header *flow.Header) error {
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
	header, err := bp.builder.BuildOn(qc.BlockID, setHotstuffFields, signProposal)
	if err != nil {
		return nil, fmt.Errorf("could not build block proposal on top of %v: %w", qc.BlockID, err)
	}

	return header, nil
}
