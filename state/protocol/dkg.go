package protocol

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type DKG interface {
	Size() (uint, error)
	GroupKey() (crypto.PublicKey, error)
	Index(nodeID flow.Identifier) (uint, error)
	KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error)
}
