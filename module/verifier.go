package module

import (
	"github.com/onflow/flow-go/crypto"
)

// Verifier is responsible for generating a signature on the given message.
type Verifier interface {
	Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error)
}

// AggregatingVerifier can verify a message against a signature from either
// a single key or many keys.
type AggregatingVerifier interface {
	Verifier
	VerifyMany(msg []byte, sig crypto.Signature, keys []crypto.PublicKey) (bool, error)
}

// ThresholdVerifier can verify a message against a signature share from a
// single key or a threshold signature against many keys.
type ThresholdVerifier interface {
	Verifier
	VerifyThreshold(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error)
}
