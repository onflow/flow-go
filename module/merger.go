package module

import (
	"github.com/onflow/flow-go/crypto"
)

// Merger is responsible for combining two signatures, but it must be done
// in a cryptographically unaware way (agnostic of the byte structure of the
// signatures).
type Merger interface {
	// Join returns a combined Signature from 2 signatures.
	Join(sig1, sig2 crypto.Signature) ([]byte, error)
	// Splits a combined signature into 2 signatures.
	Split(combined []byte) (crypto.Signature, crypto.Signature, error)
}
