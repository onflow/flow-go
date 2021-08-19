// +build relic

package signature

import "github.com/onflow/flow-go/crypto"

// This implementation of `ThresholdProvider` always produces invalid signatures
// (that are inexpensively detectable as invalid) as we cannot produce valid signatures
// due to some failure in generating random beacon key during the last DKG.
// This will be replaced by consensus voting v2, which will allow a random beacon node that failed DKG
// to submit a valid vote with only staking signature.
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
