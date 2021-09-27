package module

import (
	"errors"

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

var (
	// DKGIncompleteError indicates that the node did not complete dkg at a certain view
	DKGIncompleteError = errors.New("incomplete dkg")
)

// ThresholdSignerStore returns the threshold signer object by view
type ThresholdSignerStore interface {
	// It returns:
	//  - (signer, nil) if DKG was completed in the epoch of the view
	//  - (nil, DKGIncompleteError) if DKG was not completed in the epoch of the view
	//  - (nil, error) if there is any exception
	GetThresholdSigner(view uint64) (ThresholdSigner, error)
}
