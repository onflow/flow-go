package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

// DKG is a signer capable of generating and verifying signature shares, as well
// as reconstructing a threshold signature from shares and verifying it against
// the group public key.
type DKG struct {
	hasher crypto.Hasher
	priv   crypto.PrivateKey
}

// NewDKG creates new DKG signer & verifier, using the given hasher and the
// provided private key to generate signature shares.
func NewDKG(hasher crypto.Hasher, priv crypto.PrivateKey) *DKG {
	d := &DKG{
		hasher: hasher,
		priv:   priv,
	}
	return d
}

// Sign will use the internal private key share to generate a threshold signature
// share.
func (d *DKG) Sign(msg []byte) (crypto.Signature, error) {
	return d.priv.Sign(msg, d.hasher)
}

// Verify will verify the provided signture share against the message and the provided
// public key share.
func (d *DKG) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, d.hasher)
}

// Combine will combine the provided public signature shares to attempt and reconstruct a threshold
// signature for the group of the given size. The indices represent the index for ech signature share
// within the DKG algorithm.
func (d *DKG) Combine(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error) {

	// check that we have an index for each share
	if len(shares) != len(indices) {
		return nil, fmt.Errorf("mismatching number of shares and indices (shares: %d, indices: %d)", len(shares), len(indices))
	}

	// check that we have sufficient shares to reconstruct the threshold signature
	if !crypto.EnoughShares(int(size), len(shares)) {
		return nil, fmt.Errorf("not enough signature shares (size: %d, shares: %d)", size, len(shares))
	}

	// as the crypto API uses integer indices, let's convert the slice
	converted := make([]int, 0, len(indices))
	for _, index := range indices {
		converted = append(converted, int(index))
	}

	// try to reconstruct the threshold signature using the given shares & indices
	thresSig, err := crypto.ReconstructThresholdSignature(int(size), shares, converted)
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct threshold signature: %w", err)
	}

	return thresSig, nil
}

// VerifyThreshold will verify the given threshold signature against the given message and the given
// group public key.
func (d *DKG) VerifyThreshold(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, d.hasher)
}
