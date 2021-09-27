package module

import (
	"github.com/onflow/flow-go/crypto"
)

// TODO : to delete in V2
// Signer is a simple cryptographic signer that can sign a simple message to
// generate a signature, and verify the signature against the message.
type Signer interface {
	Verifier
	Sign(msg []byte) (crypto.Signature, error)
}

// TODO : to delete in V2
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
	Reconstruct(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error)
}

type ThresholdSignerStore interface {
	GetThresholdSigner(view uint64) (ThresholdSigner, error)
}

// ThresholdSignerStoreV2 returns the threshold signer object by view
// It returns:
//  - (signer, true, nil) if DKG was completed in the epoch of the view
//  - (nil, false, nil) if DKG was not completed in the epoch of the view
//  - (nil, false, err) if there is any exception
type ThresholdSignerStoreV2 interface {
	GetThresholdSigner(view uint64) (ThresholdSigner, bool, error)
}
