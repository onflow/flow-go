package blockproducer

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals
type BlockProducer struct {
	safetyRules hotstuff.SafetyRules
	committee   hotstuff.Replicas
	builder     module.Builder
}

var _ hotstuff.BlockProducer = (*BlockProducer)(nil)

// New creates a new BlockProducer which wraps the chain compliance layer block builder
// to provide hotstuff with block proposals.
// No errors are expected during normal operation.
func New(safetyRules hotstuff.SafetyRules, committee hotstuff.Replicas, builder module.Builder) (*BlockProducer, error) {
	bp := &BlockProducer{
		safetyRules: safetyRules,
		committee:   committee,
		builder:     builder,
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

	signer := newSafetyRulesConcurrencyWrapper(bp.safetyRules)
	header, err := bp.builder.BuildOn(qc.BlockID, setHotstuffFields, signer.Sign)
	if err != nil {
		return nil, fmt.Errorf("could not build block proposal on top of %v: %w", qc.BlockID, err)
	}
	if !signer.IsSigningComplete() {
		return nil, fmt.Errorf("signer has not yet completed signing")
	}

	return header, nil
}
