package module

import (
	"github.com/onflow/flow-go/crypto"
)

// Merger is responsible for combining two signatures, but it must be done
// in a cryptographically unaware way (agnostic of the byte structure of the
// signatures).
type Merger interface {
	Join(sig1, sig2 crypto.Signature) []byte
	Split(combined []byte) (crypto.Signature, crypto.Signature, error)
}
