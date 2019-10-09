package crypto

import (
	"fmt"
)

// NewThresholdSigner creates a new instance of Threshold siger using BLS
func NewThresholdSigner(size int, currentIndex int, leaderIndex int) (*thresholdSiger, error) {
	if currentIndex >= size || leaderIndex >= size {
		return nil, cryptoError{fmt.Sprintf("Indexes of current and leader nodes must be in the correct range.")}
	}
	minSize := 3
	if size < minSize {
		return nil, cryptoError{fmt.Sprintf("Size should be larger than 3.")}
	}

	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	threshold := optimalThreshold(size)

	return &thresholdSiger{
		size:         size,
		threshold:    threshold,
		currentIndex: currentIndex,
	}, nil
}

// dkgCommon holds the common data of all DKG protocols
type thresholdSiger struct {
	size              int
	threshold         int
	currentIndex      int
	currentPrivateKey PrivateKey
	groupPublicKey    PublicKey
	nodePublicKeys    []PublicKey
}

func (s *thresholdSiger) SignShare(message []byte) Signature {
	// use currentPrivateKey to sign
	return nil
}

func (s *thresholdSiger) VerifyShare(signerIndex int, message []byte) bool {
	// use nodePublicKeys to verify
	return true
}

func (s *thresholdSiger) VerifyThresholdSignature(message []byte) bool {
	// use groupPublicKey to verify
	return true
}

func (s *thresholdSiger) ReconstructThresholdSignature(shares []Signature, nodes []int) Signature {
	// check if length is larger than t+1 (could be set to t+1)
	// Interplate at 0
	return nil
}
