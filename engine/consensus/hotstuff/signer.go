package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Signer returns a signature for the given types
type Signer interface {
	// SignVote takes a message and public key, returns a vote signature
	// view is needed to query the signer index
	SignVote(vote *types.UnsignedVote, pubKey *types.IndexedPubKey) *types.VoteSignature
}
