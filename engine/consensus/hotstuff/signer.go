package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Signer defines a component that is able to sign proposals for the core
// HotStuff algorithm. This component must have use the private key used to
// stake.
type Signer interface {

	// TODO will be changed based on https://github.com/dapperlabs/flow-go/pull/2365
	SignVote(*types.UnsignedVote, uint32) *types.Signature
	SignBlockProposal(*types.UnsignedBlockProposal, uint32) *types.Signature
}
