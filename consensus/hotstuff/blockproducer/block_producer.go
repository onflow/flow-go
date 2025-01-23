package blockproducer

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockProducer is responsible for producing new block proposals. It is a service component to HotStuff's
// main state machine (implemented in the EventHandler). The BlockProducer's central purpose is to mediate
// concurrent signing requests to its embedded `hotstuff.SafetyRules` during block production. The actual
// work of producing a block proposal is delegated to the embedded `module.Builder`.
//
// Context: BlockProducer is part of the `hostuff` package and can therefore be expected to comply with
// hotstuff-internal design patterns, such as there being a single dedicated thread executing the EventLoop,
// including EventHandler, SafetyRules, and BlockProducer. However, `module.Builder` lives in a different
// package! Therefore, we should make the least restrictive assumptions, and support concurrent signing requests
// within `module.Builder`. To minimize implementation dependencies and reduce the chance of safety-critical
// consensus bugs, BlockProducer wraps `SafetyRules` and mediates concurrent access. Furthermore, by supporting
// concurrent singing requests, we enable various optimizations of optimistic and/or upfront block production.
type BlockProducer struct {
	safetyRules hotstuff.SafetyRules
	committee   hotstuff.Replicas
	builder     module.Builder
}

var _ hotstuff.BlockProducer = (*BlockProducer)(nil)

// New creates a new BlockProducer, which mediates concurrent signing requests to the embedded
// `hotstuff.SafetyRules` during block production, delegated to `module.Builder`.
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
// Error Returns:
//   - model.NoVoteError if it is not safe for us to vote (our proposal includes our vote)
//     for this view. This can happen if we have already proposed or timed out this view.
//   - generic error in case of unexpected failure
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
	header, err := bp.builder.BuildOn(
		qc.BlockID,
		setHotstuffFields, // never returns an error
		signer.Sign,       // may return model.NoVoteError, which we handle below
	)
	if err != nil {
		if model.IsNoVoteError(err) {
			return nil, fmt.Errorf("unsafe to vote for own proposal on top of %x: %w", qc.BlockID, err)
		}
		return nil, fmt.Errorf("could not build block proposal on top of %v: %w", qc.BlockID, err)
	}
	if !signer.IsSigningComplete() {
		return nil, fmt.Errorf("signer has not yet completed signing")
	}

	return header, nil
}
