package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Signer returns a signature for the given types
type Signer interface {
	// Sign takes a message and public key, returns a vote signature
	// view is needed to query the signer index
	Sign(message types.VoteBytes, pubKey types.PubKey, signerIndex []byte) *types.VoteSignature
}
