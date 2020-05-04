// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "thresholdsign_include.h"
import "C"

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// BLS-based threshold signature on BLS 12-381 curve
// The BLS settings are the same as in the signature
// scheme defined in the package.

// the threshold value (t) if fixed for now to t = floor((n-1)/2)
// where (t+1) signatures are required to reconstruct the
// threshold signature.
// FeldmanVSS, FeldmanVSSQual and JointFeldman are
// key generators for this threshold signature scheme

// The package offers two api:
// - stateful api where a structure holds all information
//  of the threshold signature protocols and is recommended
//  to be used for safety and to reduce protocol inconsistencies.
// - stateless api where only a to reconstruct the signature
// is provided. Verifying and storing the signature shares
// has to be done outside of the library.

// thresholdSigner is part of the stateful api
// It holds the data needed for threshold signaures
type thresholdSigner struct {
	// size of the group
	size int
	// the thresold t of the scheme where (t+1) shares are
	// required to reconstruct a signature
	threshold int
	// the index of the current node
	currentIndex int
	// the current node private key (a DKG output)
	currentPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []PublicKey
	// the hasher to be used for all signatures
	hashAlgo hash.Hasher
	// the message to be signed. Siganture shares and the threshold signature
	// are verified using this message
	messageToSign []byte
	// the valid signature shares received from other nodes
	shares []byte // simulates an array of Signatures
	// (or a matrix of by bytes) to accommodate a cgo constraint
	// the list of signers corresponding to the list of shares
	signers []index
	// the threshold signature. It is equal to nil if less than (t+1) shares are
	// received
	thresholdSignature Signature
}

// NewThresholdSigner creates a new instance of Threshold signer using BLS
// hash is the hashing algorithm to be used
// size is the number of participants
func NewThresholdSigner(size int, currentIndex int, hashAlgo hash.Hasher) (*thresholdSigner, error) {
	if size < ThresholdMinSize || size > ThresholdMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d", ThresholdMinSize, ThresholdMaxSize)
	}
	if currentIndex >= size || currentIndex < 0 {
		return nil, fmt.Errorf("The current index must be between 0 and %d", size-1)
	}

	// initialize BLS settings
	_ = newBLSBLS12381()

	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	threshold := optimalThreshold(size)
	// internal list of valid signature shares
	shares := make([]byte, 0, (threshold+1)*SignatureLenBLSBLS12381)
	signers := make([]index, 0, threshold+1)

	return &thresholdSigner{
		size:               size,
		threshold:          threshold,
		currentIndex:       currentIndex,
		hashAlgo:           hashAlgo,
		shares:             shares,
		signers:            signers,
		thresholdSignature: nil,
	}, nil
}

// SetKeys sets the private and public keys needed by the threshold signature
// - groupPublicKey is the group public key corresponding to the group secret key
// - sharePublicKeys are the public key shares corresponding to the nodes private
// key shares.
// - currentPrivateKey is the current node's own private key share
// Output keys of FeldmanVSS, FeldmanVSSQual and JointFeldman protocols could
// be used as the input keys to this function.
func (s *thresholdSigner) SetKeys(currentPrivateKey PrivateKey,
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey) {

	s.currentPrivateKey = currentPrivateKey
	s.groupPublicKey = groupPublicKey
	s.publicKeyShares = sharePublicKeys
}

// SetMessageToSign sets the next message to be signed
// All signatures shares of a different message are ignored
func (s *thresholdSigner) SetMessageToSign(message []byte) {
	s.ClearShares()
	s.messageToSign = message
}

// SignShare generates a signature share using the current private key share
func (s *thresholdSigner) SignShare() (Signature, error) {
	if s.currentPrivateKey == nil {
		return nil, errors.New("The private key of the current node is not set")
	}
	// sign
	share, err := s.currentPrivateKey.Sign(s.messageToSign, s.hashAlgo)
	if err != nil {
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	// add the node own signature
	valid, err := s.AddSignatureShare(s.currentIndex, share)
	if err != nil {
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	if !valid {
		return nil, errors.New("The current node private and public keys do not match")
	}
	return share, nil
}

// VerifyShare verifies a signature share using the signer's public key
func (s *thresholdSigner) verifyShare(share Signature, signerIndex index) (bool, error) {
	if len(s.publicKeyShares)-1 < int(signerIndex) {
		return false, errors.New("The node public keys are not set")
	}

	return s.publicKeyShares[signerIndex].Verify(share, s.messageToSign, s.hashAlgo)
}

// VerifyThresholdSignature verifies a threshold signature using the group public key
func (s *thresholdSigner) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	if s.groupPublicKey == nil {
		return false, errors.New("The group public key is not set")
	}
	return s.groupPublicKey.Verify(thresholdSignature, s.messageToSign, s.hashAlgo)
}

// ClearShares clears the shares and signers lists
func (s *thresholdSigner) ClearShares() {
	s.thresholdSignature = nil
	s.signers = s.signers[:0]
	s.shares = s.shares[:0]
}

// EnoughShares checks whether there are enough shares to reconstruct a signature
func (s *thresholdSigner) EnoughShares() bool {
	// note: len(s.signers) is always <= s.threshold + 1
	return len(s.signers) == (s.threshold + 1)
}

// AddSignatureShare processes a signature share
// If the share is valid, not perviously added and the threshold is not reached yet,
// it is appended to a local list of valid shares
// The function returns true if the share is valid, false otherwise
func (s *thresholdSigner) AddSignatureShare(orig int, share Signature) (bool, error) {
	if orig >= s.size || orig < 0 {
		return false, errors.New("orig input is invalid")
	}

	verif, err := s.verifyShare(share, index(orig))
	if err != nil {
		return false, fmt.Errorf("add signature share failed: %w", err)
	}
	// check if share is valid and threshold is not reached
	if verif && !s.EnoughShares() {
		// check if the share is new
		isSeen := false
		for _, e := range s.signers {
			if e == index(orig) {
				isSeen = true
				break
			}
		}
		if !isSeen {
			// append the share
			s.shares = append(s.shares, share...)
			s.signers = append(s.signers, index(orig))
		}
	}
	return verif, nil
}

// ThresholdSignature returns the threshold signature if the threshold was reached, nil otherwise
func (s *thresholdSigner) ThresholdSignature() (Signature, error) {
	// thresholdSignature is only computed once
	if s.thresholdSignature != nil {
		return s.thresholdSignature, nil
	}
	// reconstruct the threshold signature
	if s.EnoughShares() {
		thresholdSignature, err := s.reconstructThresholdSignature()
		if err != nil {
			return nil, err
		}
		s.thresholdSignature = thresholdSignature
		return thresholdSignature, nil
	}
	return nil, errors.New("The number of signatures shares does not reach the threshold")
}

// ReconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *thresholdSigner) reconstructThresholdSignature() (Signature, error) {
	// sanity check
	if len(s.shares) != len(s.signers)*signatureLengthBLSBLS12381 {
		s.ClearShares()
		return nil, errors.New("The number of signature shares is not matching the number of signers")
	}
	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&s.shares[0]),
		(*C.uint8_t)(&s.signers[0]), (C.int)(len(s.signers)),
	)

	// Verify the computed signature
	verif, err := s.VerifyThresholdSignature(thresholdSignature)
	if err != nil {
		return nil, fmt.Errorf("verify threshold signature failed: %w", err)
	}
	if !verif {
		return nil, errors.New(
			"The constructed threshold signature in incorrect. There might be an issue with the set keys")
	}

	return thresholdSignature, nil
}

// ReconstructThresholdSignature is a stateless api that takes a list of
// signatures and their signers's indices and returns the threshold signature.
// size is the size of the threshold signature group
// The function does not check the validity of the shares, and does not check
// the validity of the resulting signature. It also does not check the signatures signers
// are distinct.
// The function assumes the threshold value is equal to floor((n-1)/2)
// ReconstructThresholdSignature returns:
// - error if the inputs are not in the correct range or if the threshold is not reached
// - Signature: the threshold signature if there is no returned error, nil otherwise
func ReconstructThresholdSignature(size int, shares []Signature, signers []int) (Signature, error) {
	// initialize BLS settings
	_ = newBLSBLS12381()
	if size < ThresholdMinSize || size > ThresholdMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d",
			ThresholdMinSize, ThresholdMaxSize)
	}

	if len(shares) != len(signers) {
		return nil, errors.New("The number of signature shares is not matching the number of signers")
	}
	// check if the threshold was not reached
	threshold := optimalThreshold(size)
	if len(shares) < threshold+1 {
		return nil, errors.New("The number of signatures does not reach the threshold")
	}

	// flatten the shares (required by the C layer)
	flatShares := make([]byte, 0, signatureLengthBLSBLS12381*(threshold+1))
	indexSigners := make([]index, 0, threshold+1)
	for i, share := range shares {
		flatShares = append(flatShares, share...)
		if signers[i] >= size || signers[i] < 0 {
			return nil, fmt.Errorf("signer index #%d is invalid", i)
		}
		indexSigners = append(indexSigners, index(signers[i]))
	}

	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&flatShares[0]),
		(*C.uint8_t)(&indexSigners[0]), (C.int)(threshold+1),
	)
	return thresholdSignature, nil
}

// EnoughShares is a stateless function that takes the size of the threshold
// signature group and a shares number and returns true if the shares number
// is enough to reconstruct a threshold signature
// The function assumes the threshold value is equal to floor((n-1)/2)
func EnoughShares(size int, sharesNumber int) bool {
	return sharesNumber >= (optimalThreshold(size) + 1)
}


func ThresholdSignKeyGen(size int) ([]PrivateKey, 
	[]PublicKey, PublicKey,	error) {
	// set threshold 
	threshold := optimalThreshold(size)
	// initialize BLS settings
	_ = newBLSBLS12381()
	// the scalars x and G2 points y
	x := make([]scalar, size)
	y := make([]pointG2, size)
	var X0 pointG2
	// Generate a polyomial P in Zr[X] of degree t
	a := make([]scalar, threshold+1)
	for i := 0; i < threshold+1; i++ {
 		err := randZr(&a[i])
		if err != nil {
			return nil, nil, nil, err
		}
	}
	// compute the shares
	for i := index(1); int(i) <= size; i++ {
		data := make([]byte, PrKeyLenBLSBLS12381)
		zrPolynomialImage(data, a, i, &y[i-1])
		C.bn_read_bin((*C.bn_st)(&x[i-1]),(*C.uchar)(&data[0]),	PrKeyLenBLSBLS12381)
	}
	// group public key
	genScalarMultG2(&X0, &a[0])
	// export the keys
	skShares := make([]PrivateKey, size)
	pkShares := make([]PublicKey, size)
	var pkGroup PublicKey
	for i := 0; i < size; i++ {
		skShares[i] = &PrKeyBLSBLS12381{
			scalar: x[i], 
		}
		pkShares[i] = &PubKeyBLSBLS12381{
			point: y[i], 
		}
	}
	pkGroup = &PubKeyBLSBLS12381{
		point: X0, 
	}
	return skShares, pkShares, pkGroup, nil
}

