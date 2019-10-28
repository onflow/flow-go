package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "thresholdsign_include.h"
import "C"

import (
	"fmt"
)

// ThresholdSigner holds the data needed for threshold signaures
type ThresholdSigner struct {
	// size of the group
	size int
	// the thresold t of the scheme where (t+1) shares are
	// required to reconstruct a signature
	threshold int
	// the current node private key (a DKG output)
	currentPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	nodePublicKeys []PublicKey
	// the hasher to be used for all signatures
	hashAlgo Hasher
	// the message to be signed. Siganture shares and the threshold signature
	// are verified using this message
	messageToSign []byte
	// the valid signature shares received from other nodes
	shares []byte // simulates an array of Signatures
	// (or a matrix of by bytes) to accommodate a cgo constraint
	// the list of signers corresponding to the list of shares
	signers []uint32
	// the threshold signature. It is equal to nil if less than (t+1) shares are
	// received
	thresholdSignature Signature
}

// NewThresholdSigner creates a new instance of Threshold signer using BLS
// hash is the hashing algorithm to be used
// size is the number of participants
func NewThresholdSigner(size int, hash AlgoName) (*ThresholdSigner, error) {
	if size < ThresholdMinSize {
		return nil, cryptoError{fmt.Sprintf("Size should be larger than %d.",ThresholdMinSize)}
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

	return &ThresholdSigner{
		size:               size,
		threshold:          threshold,
		hashAlgo:           hasher,
		shares:             shares,
		signers:            signers,
		thresholdSignature: nil,
	}, nil
}

// SetKeys sets the private and public keys needed by the threshold signature
// the input keys can be the output keys of a Distributed Key Generator
func (s *ThresholdSigner) SetKeys(currentPrivateKey PrivateKey,
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey) {

	s.currentPrivateKey = currentPrivateKey
	s.groupPublicKey = groupPublicKey
	s.nodePublicKeys = sharePublicKeys
}

// SetMessageToSign sets the next message to be signed
// all received signatures of a different message are ignored
func (s *ThresholdSigner) SetMessageToSign(message []byte) {
	s.ClearShares()
	s.messageToSign = message
}

// SignShare generates a signature share using the current private key share
func (s *ThresholdSigner) SignShare() (Signature, error) {
	if s.currentPrivateKey == nil {
		return nil, cryptoError{"The private key of the current node is not set"}
	}
	// TOD0: should ReceiveThresholdSignatureMsg be called ?
	return s.currentPrivateKey.Sign(s.messageToSign, s.hashAlgo)
}

// VerifyShare verifies a signature share using the signer's public key
func (s *ThresholdSigner) verifyShare(share Signature, signerIndex int) (bool, error) {
	if s.size-1 < signerIndex {
		return false, cryptoError{"The signer index is larger than the group size"}
	}
	if len(s.nodePublicKeys)-1 < signerIndex {
		return false, cryptoError{"The node public keys are not set"}
	}

	return s.nodePublicKeys[signerIndex].Verify(share, s.messageToSign, s.hashAlgo)
}

// VerifyThresholdSignature verifies a threshold signature using the group public key
func (s *ThresholdSigner) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	if s.groupPublicKey == nil {
		return false, cryptoError{"The group public key is not set"}
	}
	return s.groupPublicKey.Verify(thresholdSignature, s.messageToSign, s.hashAlgo)
}

// ClearShares clears the shares and signers lists
func (s *ThresholdSigner) ClearShares() {
	s.thresholdSignature = nil
	s.signers = s.signers[:0]
	s.shares = s.shares[:0]
}

// ReceiveThresholdSignatureMsg processes a new TS message received by the current node
func (s *ThresholdSigner) ReceiveThresholdSignatureMsg(orig int, share Signature) (bool, Signature, error) {
	// if origin is disqualified, ignore the message
	// a desqualified node is encoded with a nil public key
	if s.nodePublicKeys[orig] == nil {
		return false, nil, nil
	}

	verif, err := s.verifyShare(share, orig)
	if err != nil {
		return false, nil, err
	}
	if verif {
		s.shares = append(s.shares, share...)
		s.signers = append(s.signers, uint32(orig))
		// thresholdSignature is only computed once
		if len(s.signers) == (s.threshold + 1) {
			thresholdSignature, err := s.reconstructThresholdSignature()
			if err != nil {
				return true, nil, err
			}
			s.thresholdSignature = thresholdSignature
		}
	}
	return verif, s.thresholdSignature, nil
}

// ReconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *ThresholdSigner) reconstructThresholdSignature() (Signature, error) {
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

	// Verify the computed signature
	verif, err := s.VerifyThresholdSignature(thresholdSignature)
	if err != nil {
		return nil, err
	}
	if !verif {
		return nil, cryptoError{
			"The constructed threshold signature in incorrect. There might be an issue with the set keys"}
	}

	return thresholdSignature, nil
}
