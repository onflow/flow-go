package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/module"
)

// AggregationVerifier is an aggregating verifier that can verify signatures and
// verify aggregated signatures.
// *Important: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationVerifier struct {
	hasher hash.Hasher
}

// NewAggregationVerifier creates a new aggregation verifier, which can only
// verify signatures. *Important*: the aggregation verifier can only verify
// signatures in the context of the provided KMAC tag.
func NewAggregationVerifier(tag string) *AggregationVerifier {
	av := &AggregationVerifier{
		hasher: crypto.NewBLS_KMAC(tag),
	}
	return av
}

// Verify will verify the given signature against the given message and public key.
func (av *AggregationVerifier) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, av.hasher)
}

// VerifyMany will verify the given aggregated signature against the given message and the
// provided public keys.
func (av *AggregationVerifier) VerifyMany(msg []byte, sig crypto.Signature, keys []crypto.PublicKey) (bool, error) {

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
		valid, err := av.Verify(msg, sig, keys[i])
		if err != nil {
			return false, fmt.Errorf("could not verify signature (index: %d): %w", i, err)
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}

// AggregationProvider is an aggregating signer and verifier that can create/verify
// signatures, as well as aggregating & verifying aggregated signatures.
// *Important*: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationProvider struct {
	*AggregationVerifier
	local module.Local
}

// NewAggregationProvider creates a new aggregation provider using the given private
// key to generate signatures. *Important*: the aggregation provider can only
// create and verify signatures in the context of the provided HMAC tag.
func NewAggregationProvider(tag string, local module.Local) *AggregationProvider {
	ap := &AggregationProvider{
		AggregationVerifier: NewAggregationVerifier(tag),
		local:               local,
	}
	return ap
}

// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (ap *AggregationProvider) Sign(msg []byte) (crypto.Signature, error) {
	return ap.local.Sign(msg, ap.hasher)
}

// Aggregate will aggregate the given signatures into one aggregated signature.
func (ap *AggregationProvider) Aggregate(sigs []crypto.Signature) (crypto.Signature, error) {

	// NOTE: the current implementation simply concatenates all signatures; this
	// will be replace by real AggregationProvider signature aggregation once available
	c := &Combiner{}
	sig, err := c.Join(sigs...)
	if err != nil {
		return nil, fmt.Errorf("could not combine signatures: %w", err)
	}

	return sig, nil
}
