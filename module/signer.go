package module

import (
	"errors"

	"github.com/onflow/flow-go/crypto"
)

// TODO : to be removed
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
	// DKGFailError indicates that the node has completed DKG, but failed to genereate private key
	// in the given epoch
	DKGFailError = errors.New("dkg failed, no DKG private key generated")
)

// TODO: to be replaced by RandomBeaconKeyStore
// ThresholdSignerStore returns the threshold signer object by view
type ThresholdSignerStore interface {
	// It returns:
	//  - (signer, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGFailError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	GetThresholdSigner(view uint64) (ThresholdSigner, error)
}

// RandomBeaconKeyStore returns the random beacon private key for the given view,
type RandomBeaconKeyStore interface {
	// It returns:
	//  - (signer, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGFailError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	ByView(view uint64) (crypto.PrivateKey, error)
}
