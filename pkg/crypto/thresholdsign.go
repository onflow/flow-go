package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "thresholdsign_include.h"
import "C"

// TODO: remove -wall after reaching a stable version
// TDOD: enable QUIET in relic

import (
	"fmt"
)

// NewThresholdSigner creates a new instance of Threshold siger using BLS
// The hashing algorithm to be used is passed as a parameter
func NewThresholdSigner(size int, hash AlgoName) (*ThresholdSinger, error) {
	if size < ThresholdMinSize {
		return nil, cryptoError{fmt.Sprintf("Size should be larger than 3.")}
	}

	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	threshold := optimalThreshold(size)
	// Hahser to be used
	hasher, err := NewHasher(hash)
	if err != nil {
		return nil, err
	}
	shares := make([]byte, 0, size*signatureLengthBLS_BLS12381)
	signers := make([]uint32, 0, size)

	return &ThresholdSinger{
		size:               size,
		threshold:          threshold,
		hashAlgo:           hasher,
		shares:             shares,
		signers:            signers,
		thresholdSignature: nil,
	}, nil
}

// ThresholdSinger holds the data needed for threshold signaures
type ThresholdSinger struct {
	size              int
	threshold         int
	currentPrivateKey PrivateKey
	groupPublicKey    PublicKey
	nodePublicKeys    []PublicKey
	hashAlgo          Hasher
	messageToSign     []byte
	shares            []byte // simulates an array of Signatures
	// (or a matrix of by bytes) to solve a cgo issue
	signers            []uint32
	thresholdSignature Signature
}

// Resize sets update the size and the threshold of the group
func (s *ThresholdSinger) Resize(newSize int) error {
	if newSize < ThresholdMinSize {
		return cryptoError{fmt.Sprintf("Size should be larger than 3.")}
	}
	s.size = newSize
	s.threshold = optimalThreshold(newSize)
	return nil
}

// SetKeys sets the private and public keys needed by the threshold signature
// the input keys could be the output keys of a Distributed Key Generator
func (s *ThresholdSinger) SetKeys(currentPrivateKey PrivateKey,
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey) {

	s.currentPrivateKey = currentPrivateKey
	s.groupPublicKey = groupPublicKey
	s.nodePublicKeys = sharePublicKeys
}

// SetMessageToSign sets the next message to be signed
// all received signatures of a different message are ignored
func (s *ThresholdSinger) SetMessageToSign(message []byte) {
	s.ClearShares()
	s.messageToSign = message
}

// SignShare generates a signature share using the current private key share
func (s *ThresholdSinger) SignShare() (Signature, error) {
	if s.currentPrivateKey == nil {
		return nil, cryptoError{"The private key of the current node is not set"}
	}
	// call receiveshare?
	return s.currentPrivateKey.Sign(s.messageToSign, s.hashAlgo)
}

// VerifyShare verifies a signature share using the signer's public key
func (s *ThresholdSinger) verifyShare(share Signature, signerIndex int) (bool, error) {
	if s.size-1 < signerIndex {
		return false, cryptoError{"The signer index is larger than the group size"}
	}
	if len(s.nodePublicKeys)-1 < signerIndex {
		return false, cryptoError{"The node public keys are not set"}
	}

	return s.nodePublicKeys[signerIndex].Verify(share, s.messageToSign, s.hashAlgo)
}

// VerifyThresholdSignature verifies a threshold signature using the group public key
func (s *ThresholdSinger) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	if s.groupPublicKey == nil {
		return false, cryptoError{"The group public key is not set"}
	}
	return s.groupPublicKey.Verify(thresholdSignature, s.messageToSign, s.hashAlgo)
}

// ClearShares clears the shares and signers lists
func (s *ThresholdSinger) ClearShares() {
	s.thresholdSignature = nil
	s.signers = s.signers[:0]
	s.shares = s.shares[:0]
}

// ReceiveThresholdSignatureMsg processes a new TS message received by the current node
func (s *ThresholdSinger) ReceiveThresholdSignatureMsg(orig int, share Signature) (bool, Signature, error) {
	// if origin is disqualified, ignore the message
	if s.nodePublicKeys[orig] == nil {
		return false, nil, nil
	}

	verif, err := s.verifyShare(Signature(share), orig)
	if err != nil {
		return false, nil, err
	}
	if verif {
		s.shares = append(s.shares, share...)
		s.signers = append(s.signers, uint32(orig))
		// thresholdSignature is only computed once
		if len(s.signers) == (s.threshold + 1) {
			fmt.Println(s.signers)
			thresholdSignature, err := s.reconstructThresholdSignature()
			if err != nil {
				return true, nil, err
			}
			s.thresholdSignature = thresholdSignature
			fmt.Println(thresholdSignature)
		}
	}
	return verif, s.thresholdSignature, nil
}

// ReconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *ThresholdSinger) reconstructThresholdSignature() (Signature, error) {
	// sanity check
	if len(s.shares) != len(s.signers)*signatureLengthBLS_BLS12381 {
		s.ClearShares()
		return nil, cryptoError{"The number of signature shares is not matching the number of signers"}
	}
	thresholdSignature := make([]byte, signatureLengthBLS_BLS12381)
	// Lagrange Interpolate at point 0
	C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&s.shares[0]),
		(*C.uint32_t)(&s.signers[0]), (C.int)(len(s.signers)),
	)
	return thresholdSignature, nil
}
