package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
)

// RandomBeaconSigner encapsulates all methods needed by a Hotstuff leader to validate the beacon votes and reconstruct a beacon signature.
// The random beacon methods are based on a threshold signature scheme.
type RandomBeaconSigner interface {
	// Verify verifies the signature share under the signer's public key and the message agreed upon.
	// It allows concurrent verification of the given signature.
	// It returns nil if signature is valid,
	// crypto.InvalidInputsError if the signature is invalid,
	// and other error if there is an exception.
	Verify(signerIndex int, share crypto.Signature) error

	// TrustedAdd adds a share to the internal signature shares store.
	// The operation is sequential.
	// The function does not verify the signature is valid. It is the caller's responsibility
	// to make sure the signature was previously verified.
	// It returns:
	// (true, nil) if the signature has been added, and enough shares have been collected.
	// (false, nil) if the signature has been added, but not enough shares were collected.
	// (false, error) if there is any exception adding the signature share.
	TrustedAdd(signerIndex int, share crypto.Signature) (enoughshares bool, exception error)

	// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
	// a group signature.
	EnoughShares() bool

	// SignShare produces a signature share with the internal participant's private key.
	SignShare() (crypto.Signature, error)

	// Reconstruct reconstructs the group signature.
	// The reconstructed signature is verified against the overall group public key and the message agreed upon.
	// This is a sanity check that is necessary since "TrustedAdd" allows adding non-verified signatures.
	// Reconstruct returns an error if the reconstructed signature fails the sanity verification, or if not enough shares have been collected.
	Reconstruct() (crypto.Signature, error)
}

// This provides a mock implementation of RandomBeaconSigner in crypto/thresholdsign.go
// TODO: remove when crypto/thresholdsign supports concurrent calls.
type ThresholdSignerImpl struct {
	// dependencies
	msg             []byte
	threshold       int
	groupPublicKey  crypto.PublicKey
	myIndex         int
	myPrivateKey    crypto.PrivateKey
	publicKeyShares []crypto.PublicKey

	// state
	haveEnoughShares   bool
	signers            []int
	thresholdSignature crypto.Signature
}

func NewThresholdSignerImpl(
	msg []byte,
	threshold int,
	groupPublicKey crypto.PublicKey,
	myIndex int,
	myPrivateKey crypto.PrivateKey,
	publicKeyShares []crypto.PublicKey) *ThresholdSignerImpl {
	return &ThresholdSignerImpl{
		msg:             msg,
		threshold:       threshold,
		groupPublicKey:  groupPublicKey,
		myIndex:         myIndex,
		myPrivateKey:    myPrivateKey,
		publicKeyShares: publicKeyShares,

		haveEnoughShares:   false,
		signers:            nil,
		thresholdSignature: nil,
	}
}
