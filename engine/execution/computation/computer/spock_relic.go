//go:build relic
// +build relic

package computer

import (
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
)

// This is a temporary wrapper that around the crypto library.
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
	return crypto.SPOCKProve(sk, data, kmac)
}
