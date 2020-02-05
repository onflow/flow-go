package types

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// PubKey is the public key type for a flow identity, should be the same type as
// flow.Identity's PubKey field
type PubKey = crypto.PublicKey
