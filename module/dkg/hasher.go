package dkg

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
)

// NewDKGMessageHasher returns a hasher for signing and verifying DKG broadcast
// messages.
func NewDKGMessageHasher() hash.Hasher {
	return crypto.NewBLSKMAC(encoding.DKGMessageTag)
}
