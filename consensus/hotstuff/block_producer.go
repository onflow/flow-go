package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockProducer builds a new block proposal by building a new block payload with the builder module,
// and uses VoteCollectorFactory to create a disposable VoteCollector for producing the proposal vote.
// BlockProducer assembles the new block proposal using the block payload, block header and the proposal vote.
type BlockProducer interface {
	// MakeBlockProposal builds a new HotStuff block proposal using the given view,
	// the given quorum certificate for its parent and [maybe] timeout certificate for last view(could be nil).
	// No errors are expected during normal operation.
	MakeBlockProposal(qc *flow.QuorumCertificate, view uint64, lastViewTC *flow.TimeoutCertificate) (*model.Proposal, error)
}
