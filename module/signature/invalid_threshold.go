// +build relic

package signature

import "github.com/onflow/flow-go/crypto"

// This implementation of `ThresholdProvider` always produces invalid signatures
// (that are inexpensively detectable as invalid) as we cannot produce valid signatures.
// Used when this node fails to generate a random beacon key during the last DKG.
// This will be replaced by consensus voting v2, which will be able to gracefully
// handle consensus nodes submitting votes with only one signature.
type InvalidThresholdProvider struct {
	*ThresholdVerifier
}

// NewInvalidThresholdProvider returns a new instance of `InvalidThresholdProvider`
func NewInvalidThresholdProvider(tag string) *InvalidThresholdProvider {
	tp := &InvalidThresholdProvider{
		ThresholdVerifier: NewThresholdVerifier(tag),
	}
	return tp
}

// Sign always returns an invalid signature
func (tp *InvalidThresholdProvider) Sign(msg []byte) (crypto.Signature, error) {
	return crypto.BLSInvalidSignature(), nil
}

// Reconstruct always returns an invalid signature
func (tp *InvalidThresholdProvider) Reconstruct(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error) {
	return crypto.BLSInvalidSignature(), nil
}
