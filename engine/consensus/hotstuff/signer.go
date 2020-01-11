package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Signer interface {
	SignVote(*types.Vote, uint32) *types.Vote
	SignBlockProposal(*types.BlockProposal, uint32) *types.BlockProposal
	// SignChallenge()
}

