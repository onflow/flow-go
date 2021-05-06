// +build relic

package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/module"
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
		hasher: crypto.NewBLSKMAC(tag),
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

	// bls multi-signature verification with a single message
	valid, err := crypto.VerifyBLSSignatureOneMessage(keys, sig, msg, av.hasher)
	if err != nil {
		return false, fmt.Errorf("could not verify aggregated signature: %w", err)
	}

	return valid, nil
}

// AggregationAggregator is for aggregating signatures.
type AggregationAggregator struct{}

// NewAggregationAggregator returns a new AggregationAggregator.
func NewAggregationAggregator() *AggregationAggregator {
	return &AggregationAggregator{}
}

// Aggregate will aggregate the given signatures into one aggregated signature.
func (ap *AggregationAggregator) Aggregate(sigs []crypto.Signature) (crypto.Signature, error) {

	// BLS aggregation
	sig, err := crypto.AggregateBLSSignatures(sigs)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate BLS signatures: %w", err)
	}
	return sig, nil
}

// AggregationProvider is an aggregating signer and verifier that can create/verify
// signatures, as well as aggregating & verifying aggregated signatures.
// *Important*: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationProvider struct {
	*AggregationVerifier
	*AggregationAggregator
	local module.Local
}

// NewAggregationProvider creates a new aggregation provider using the given private
// key to generate signatures. *Important*: the aggregation provider can only
// create and verify signatures in the context of the provided HMAC tag.
func NewAggregationProvider(tag string, local module.Local) *AggregationProvider {
	ap := &AggregationProvider{
		AggregationVerifier:   NewAggregationVerifier(tag),
		AggregationAggregator: NewAggregationAggregator(),
		local:                 local,
	}
	return ap
}

// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (ap *AggregationProvider) Sign(msg []byte) (crypto.Signature, error) {
	return ap.local.Sign(msg, ap.hasher)
}
