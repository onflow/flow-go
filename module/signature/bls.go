package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

// BLS is an aggregating signer and verifier that can create/verify BLS
// signatures, as well as aggregating & verifying aggregated BLS signatures.
type BLS struct {
	hasher crypto.Hasher
	priv   crypto.PrivateKey
}

// NewBLS creates a new BLS signer and verifier, using the given hasher and the
// provided private key for creating signatures.
func NewBLS(hasher crypto.Hasher, priv crypto.PrivateKey) *BLS {
	b := &BLS{
		hasher: hasher,
		priv:   priv,
	}
	return b
}

// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (b *BLS) Sign(msg []byte) (crypto.Signature, error) {
	return b.priv.Sign(msg, b.hasher)
}

// Verify will verify the given signature against the given message and public key.
func (b *BLS) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, b.hasher)
}

// Aggregate will aggregate the given signatures into one aggregated signature.
func (b *BLS) Aggregate(sigs []crypto.Signature) (crypto.Signature, error) {

	// NOTE: the current implementation simply concatenates all signatures; this
	// will be replace by real BLS signature aggregation once available
	c := &Combiner{}
	sig, err := c.Join(sigs...)
	if err != nil {
		return nil, fmt.Errorf("could not combine signatures: %w", err)
	}

	return sig, nil
}

// VerifyMany will verify the given aggregated signature against the given message and the
// provided public keys.
func (b *BLS) VerifyMany(msg []byte, sig crypto.Signature, keys []crypto.PublicKey) (bool, error) {

	// NOTE: for now, we simply split the concatenated signature into its parts and verify each
	// of them separately; in the future, this will be replaced by real aggregated signature verification
	c := &Combiner{}
	sigs, err := c.Split(sig)
	if err != nil {
		return false, fmt.Errorf("could not split signatures: %w", err)
	}
	if len(keys) != len(sigs) {
		return false, fmt.Errorf("invalid number of public keys (signatures: %d, keys: %d)", len(sigs), len(keys))
	}
	for i, sig := range sigs {
		valid, err := b.Verify(msg, sig, keys[i])
		if err != nil {
			return false, fmt.Errorf("could not verify signature (index: %d): %w", i, err)
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}
