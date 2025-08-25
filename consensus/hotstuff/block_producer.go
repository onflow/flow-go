package hotstuff

import (
	"github.com/onflow/flow-go/model/flow"
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
type BlockProducer interface {
	// MakeBlockProposal builds a new HotStuff block proposal using the given view,
	// the given quorum certificate for its parent and [optionally] a timeout certificate for last view (could be nil).
	// Error Returns:
	//   - model.NoVoteError if it is not safe for us to vote (our proposal includes our vote)
	//     for this view. This can happen if we have already proposed or timed out this view.
	//   - generic error in case of unexpected failure
	MakeBlockProposal(view uint64, qc *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*flow.ProposalHeader, error)
}
