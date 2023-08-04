package dkg

import (
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/module/signature"
)

// NewDKGMessageHasher returns a hasher for signing and verifying DKG broadcast
// messages.
func NewDKGMessageHasher() hash.Hasher {
	return signature.NewBLSHasher(signature.DKGMessageTag)
}
