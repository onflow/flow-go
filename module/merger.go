package module

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// Merger is responsible for combining two signatures, but it must be done
// in a cryptographically unaware way (agnostic of the byte structure of the
// signatures).
type Merger interface {
	Join(sigs ...crypto.Signature) ([]byte, error)
	Split(combined []byte) ([]crypto.Signature, error)
}
