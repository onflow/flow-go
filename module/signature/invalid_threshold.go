// +build relic

package signature

import "github.com/onflow/flow-go/crypto"

type InvalidThresholdProvider struct {
	*ThresholdVerifier
}

func NewInvalidThresholdProvider(tag string) *InvalidThresholdProvider {
	tp := &InvalidThresholdProvider{
		ThresholdVerifier: NewThresholdVerifier(tag),
	}
	return tp
}

func (tp *InvalidThresholdProvider) Sign(msg []byte) (crypto.Signature, error) {
	return crypto.Signature{}, nil
}

func (tp *InvalidThresholdProvider) Reconstruct(size uint, shares []crypto.Signature, indices []uint) (crypto.Signature, error) {
	return crypto.Signature{}, nil
}
