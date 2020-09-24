package module

import (
	"github.com/onflow/flow-go/crypto"
)

// Signer is a simple cryptographic signer that can sign a simple message to
// generate a signature, and verify the signature against the message.
type Signer interface {
	Verifier
	Sign(msg []byte) (crypto.Signature, error)
}

// AggregatingSigner is a signer that can sign a simple message and aggregate
// multiple signatures into a single aggregated signature.
type AggregatingSigner interface {
	AggregatingVerifier
	Sign(msg []byte) (crypto.Signature, error)
	Aggregate(sigs []crypto.Signature) (crypto.Signature, error)
}

// ThresholdSigner is a signer that can sign a message to generate a signature
// share and construct a threshold signature from the given shares.
type ThresholdSigner interface {
	ThresholdVerifier
	Sign(msg []byte) (crypto.Signature, error)
	Combine(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error)
}
