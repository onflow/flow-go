//go:build !relic
// +build !relic

package computer

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

// This is a temporary wrapper that simulates a call to SPoCK prove,
// required for the emulator build. The function is never called by the
// emulator although it is required for a successful build.
//
// TODO(tarak): remove once the crypto module properly implements a non-relic
// version of SPOCKProve.
func SPOCKProve(
	sk crypto.PrivateKey,
	data []byte,
	kmac hash.Hasher,
) (
	crypto.Signature,
	error,
) {
	panic("SPoCK prove not supported when flow-go is built without relic")
}
