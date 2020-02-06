package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have use the private key used to
// stake.
type Signer interface {
	SignVote(*types.UnsignedVote, types.PubKey) *flow.PartialSignature
}
