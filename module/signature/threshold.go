// +build relic

package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

// ThresholdVerifier is a verifier capable of verifying threshold signature
// shares and verifying a threshold signature against a group public key.
// *Important*: the threshold verifier can only verify signatures for
// messages of one specific context (specified via the KMAC tag at construction).
type ThresholdVerifier struct {
	hasher hash.Hasher
}

// RandomBeaconThreshold returns the threshold (t) to allow the largest number of
// malicious nodes (m) assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func RandomBeaconThreshold(size int) int {
	return (size - 1) / 2
}

// NewThresholdVerifier creates a new threshold verifier. *Important*: the
// threshold provider can only verify signatures in the context of the provided
// KMAC tag.
func NewThresholdVerifier(tag string) *ThresholdVerifier {
	tv := &ThresholdVerifier{
		hasher: crypto.NewBLSKMAC(tag),
	}
	return tv
}

// Verify will verify the provided signture share against the message and the provided
// public key share.
func (tv *ThresholdVerifier) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, tv.hasher)
}

// VerifyThreshold will verify the given threshold signature against the given message and the given
// group public key.
func (tv *ThresholdVerifier) VerifyThreshold(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, tv.hasher)
}

// ThresholdProvider is a signer capable of generating and verifying signature
// shares, as well as reconstructing a threshold signature from shares and
// verifying it against a group public key.
// *Important*: the threshold provider can only create and verify signatures in
// the context of the provided KMAC tag.
type ThresholdProvider struct {
	*ThresholdVerifier
	priv crypto.PrivateKey
}

// NewThresholdProvider creates new threshold provider, using the given private
// key share to generate signature shares. *Important*: the threshold provider
// can only create and verify signatures in the context of the provided KMAC tag.
func NewThresholdProvider(tag string, priv crypto.PrivateKey) *ThresholdProvider {
	tp := &ThresholdProvider{
		ThresholdVerifier: NewThresholdVerifier(tag),
		priv:              priv,
	}
	return tp
}

// Sign will use the internal private key share to generate a threshold signature
// share.
func (tp *ThresholdProvider) Sign(msg []byte) (crypto.Signature, error) {
	return tp.priv.Sign(msg, tp.hasher)
}

// Combine will combine the provided public signature shares to attempt and reconstruct a threshold
// signature for the group of the given size. The indices represent the index for ech signature share
// within the DKG algorithm.
func (tp *ThresholdProvider) Combine(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error) {

	// check that we have sufficient shares to reconstruct the threshold signature
	if !crypto.EnoughShares(RandomBeaconThreshold(int(size)), len(shares)) {
		return nil, fmt.Errorf("not enough signature shares (size: %d, shares: %d)", size, len(shares))
	}

	// as the crypto API uses integer indices, let's convert the slice
	converted := make([]int, 0, len(indices))
	for _, index := range indices {
		converted = append(converted, int(index))
	}

	// try to reconstruct the threshold signature using the given shares & indices
	thresSig, err := crypto.ReconstructThresholdSignature(int(size), RandomBeaconThreshold(int(size)), shares, converted)
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct threshold signature: %w", err)
	}

	return thresSig, nil
}
