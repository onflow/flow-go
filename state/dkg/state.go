package dkg

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// State represents the state of the DKG system.
type State interface {
	GroupSize() (uint, error)
	GroupKey() (crypto.PublicKey, error)
	ShareIndex(nodeID flow.Identifier) (uint, error)
	ShareKey(nodeID flow.Identifier) (crypto.PublicKey, error)
}
