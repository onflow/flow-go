package module

import (
	"errors"

	"github.com/onflow/flow-go/crypto"
)

// TODO : to replaced by MsgSigner in V2
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

// TODO: to delete in V2
// ThresholdSigner is a signer that can sign a message to generate a signature
// share and construct a threshold signature from the given shares.
type ThresholdSigner interface {
	ThresholdVerifier
	Sign(msg []byte) (crypto.Signature, error)
	Reconstruct(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error)
}

var (
	// DKGIncompleteError indicates that the node did not manage to obtain beacon keys from DKG at a certain view
	DKGIncompleteError = errors.New("incomplete dkg")
)

// TODO: to be replaced by RandomBeaconSignerStore
// ThresholdSignerStore returns the threshold signer object by view
type ThresholdSignerStore interface {
	// It returns:
	//  - (signer, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGIncompleteError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	GetThresholdSigner(view uint64) (ThresholdSigner, error)
}

// MsgSigner signs a given message and produces a signature
// TODO: could be renamed to Signer once the original Signer is replaced with MsgSigner
type MsgSigner interface {
	Sign(msg []byte) (crypto.Signature, error)
}

// RandomBeaconSignerStore returns the signer for the given view, and it internally caches
// signer objects by epoch. And epoch is determined by view.
type RandomBeaconSignerStore interface {
	// It returns:
	//  - (signer, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGIncompleteError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	GetSigner(view uint64) (MsgSigner, error)
}
