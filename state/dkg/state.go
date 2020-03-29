package dkg

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type State interface {
	GroupKey() (crypto.PublicKey, error)
	GroupSize() (uint, error)
	ShareIndex(nodeID flow.Identifier) (uint, error)
	ShareKey(nodeID flow.Identifier) (crypto.PublicKey, error)
}
