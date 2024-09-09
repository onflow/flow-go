package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Signer is responsible for creating votes, proposals for a given block.
type Signer interface {
	// CreateVote creates a vote for the given block. No error returns are
	// expected during normal operations (incl. presence of byz. actors).
	CreateVote(block *model.Block) (*model.Vote, error)

	// CreateTimeout creates a timeout for given view. No errors return are
	// expected during normal operations(incl presence of byz. actors).
	CreateTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error)
}
