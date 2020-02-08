package signature

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Signer returns a signature for the given types
type Signer interface {
	// SignVote signs a vote, returns a vote signature
	SignVote(vote *types.UnsignedVote) *types.Signature
	// SignBlock signs a block, returns a block signature
	SignBlock(block *types.Block) *types.Signature
}
