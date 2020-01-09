package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Signer interface {
	SignVote(*types.Vote) *types.Vote
	SignBlockProposal(*types.BlockProposal) *types.BlockProposal
	// SignChallenge()
}
