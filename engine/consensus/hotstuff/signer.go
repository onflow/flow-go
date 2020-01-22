package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Signer returns a signature for the given types
type Signer interface {
	SignVote(*types.Vote, uint32) *types.Signature
	SignBlockProposal(*types.BlockProposal, uint32) *types.Signature
	// SignChallenge()
}
