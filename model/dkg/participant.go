package dkg

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// Participant contains an individual participant's DKG data
type Participant struct {
	Index          uint
	PublicKeyShare crypto.PublicKey
}
