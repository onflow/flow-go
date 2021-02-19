package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Verifier is the component responsible for validating votes, proposals and
// QC's against the block they are based on.
type Verifier interface {

	// VerifyVote checks the validity of a vote for the given block.
	VerifyVote(voter *flow.Identity, sigData []byte, block *model.Block) (bool, error)

	// VerifyQC checks the validity of a QC for the given block.
	VerifyQC(voters flow.IdentityList, sigData []byte, block *model.Block) (bool, error)
}
