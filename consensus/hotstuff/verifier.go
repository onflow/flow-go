package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Verifier is the component responsible for validating votes, proposals and
// QC's against the block they are based on.
type Verifier interface {

	// VerifyVote checks the validity of a vote for the given block.
	VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error)

	// VerifyQC checks the validity of a QC for the given block.
	VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error)
}
