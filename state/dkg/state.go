package dkg

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// State represents the state of the DKG system.
type State interface {
	GroupSize() (uint, error)
	GroupKey() (crypto.PublicKey, error)
	ParticipantIndex(nodeID flow.Identifier) (uint, error)
	ParticipantKey(nodeID flow.Identifier) (crypto.PublicKey, error)
}
