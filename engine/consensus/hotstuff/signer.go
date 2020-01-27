package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have use the private key used to
// stake.
type Signer interface {

	// Sign generates a signature for the given vote, using the node's
	// private key.
	// TODO this is assuming we consolidate the "signable" model to the vote type
	// TODO should return a BLS partial signature
	Sign(*types.Vote) (crypto.Signature, error)
}
