package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Signer returns a signature for the given types
type Signer interface {
	SignVote(*types.UnsignedVote, types.PubKey) *types.Signature
}
