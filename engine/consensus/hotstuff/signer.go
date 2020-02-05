package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Signer returns a signature for the given types
type Signer interface {
	// SignVote signs a vote with the signer index, returns a vote signature
	// vote - the vote to be signed
	// signerIndex - the index of the signer in its cluster. The signerIndex
	// along with the BlockID field in the vote, determines the public key,
	// of which the private key will be used to sign the vote.
	SignVote(vote *types.UnsignedVote, signerIndex SignerIndex) *types.VoteSignature
}
