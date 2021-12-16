package module

import (
	"github.com/onflow/flow-go/crypto"
)

// TODO : to delete in V2
// Verifier is responsible for verifying a signature on the given message.
type Verifier interface {
	Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error)
}
