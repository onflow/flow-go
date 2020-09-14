package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockProducer is the component that is responsible for producing a new block
// proposal for HotStuff when we are the leader for a view.
type BlockProducer interface {

	// MakeBlockProposal builds a new HotStuff block proposal using the given view and
	// the given quorum certificate for its parent.
	MakeBlockProposal(qc *flow.QuorumCertificate, view uint64) (*model.Proposal, error)
}
