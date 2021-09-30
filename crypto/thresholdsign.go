// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "thresholdsign_include.h"
import "C"

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto/hash"
)

// BLS-based threshold signature on BLS 12-381 curve
// The BLS settings are the same as in the signature
// scheme defined in the package.

// A threshold signature scheme allows any subset of (t+1)
// valid signature shares to reconstruct the threshold signature.
// up to (t) shares do not reveal any information about the threshold
// signature.
// Although the API allows using arbitrary values of (t),
// the threshold signature scheme is secure in the presence of up to (t)
// malicious participants when (t < n/2).
// In order to optimize equally for unforgeability and robustness,
// the input threshold value (t) should be set to t = floor((n-1)/2).

// The package offers two api:
// - stateful api where a structure holds all information
//  of the threshold signature protocols and is recommended
//  to be used for safety and to reduce protocol inconsistencies.
// - stateless api with signature reconstruction. Verifying and storing
// the signature shares has to be managed outside of the library.

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
	// the current node private key (a threshold KG output)
	currentPrivateKey PrivateKey
	// the group public key (a threshold KG output)
	groupPublicKey PublicKey
	// the group public key shares (a threshold KG output)
	publicKeyShares []PublicKey
	// the hasher to be used for all signatures
	hashAlgo hash.Hasher
	// the message to be signed. Siganture shares and the threshold signature
	// are verified against this message
	message []byte
	// the valid signature shares received from other participants
	shares map[index]Signature
	// the threshold signature. It is equal to nil if less than (t+1) shares are
	// received
	thresholdSignature Signature
	// lock for atomic operations
	lock sync.RWMutex
}

// NewThresholdSigner creates a new instance of Threshold signer using BLS.
// hash is the hashing algorithm to be used.
// size is the number of participants, it must be in the range [ThresholdSignMinSize..ThresholdSignMaxSize]
// threshold is the threshold value, it must be in the range [MinimumThreshold..size-1]
func NewThresholdSigner(
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey,
	threshold int,
	currentIndex int,
	currentPrivateKey PrivateKey,
	message []byte,
	hashAlgo hash.Hasher) (*thresholdSigner, error) {

	size := len(sharePublicKeys)
	if size < ThresholdSignMinSize || size > ThresholdSignMaxSize {
		return nil, newInvalidInputsError(
			"size should be between %d and %d, got %d",
			ThresholdSignMinSize, ThresholdSignMaxSize, size)
	}
	if currentIndex >= size || currentIndex < 0 {
		return nil, newInvalidInputsError(
			"the current index must be between 0 and %d, got %d",
			size-1, currentIndex)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, newInvalidInputsError(
			"the threshold must be between %d and %d, got %d",
			MinimumThreshold, size-1, threshold)
	}

	// set BLS settings
	blsInstance.reInit()

	// check keys are BLS keys
	for i, pk := range sharePublicKeys {
		if _, ok := pk.(*PubKeyBLSBLS12381); !ok {
			return nil, newInvalidInputsError("key at index %d is not a BLS key", i)
		}
	}
	if _, ok := groupPublicKey.(*PubKeyBLSBLS12381); !ok {
		return nil, newInvalidInputsError("group key at is not a BLS key")
	}
	if _, ok := currentPrivateKey.(*PrKeyBLSBLS12381); !ok {
		return nil, fmt.Errorf("the private key of node %d is not a BLS key", currentIndex)
	}

	// check the private key, index and corresponding public key are consistent
	currentPublicKey := sharePublicKeys[currentIndex]
	if !currentPrivateKey.PublicKey().Equals(currentPublicKey) {
		return nil, newInvalidInputsError("private key is not matching public key at index %d", currentIndex)
	}

	// internal list of valid signature shares
	shares := make(map[index]Signature)

	return &thresholdSigner{
		size:               size,
		threshold:          threshold,
		currentIndex:       currentIndex,
		message:            message,
		hashAlgo:           hashAlgo,
		shares:             shares,
		thresholdSignature: nil,
		currentPrivateKey:  currentPrivateKey, // currentPrivateKey is the current node's own private key share
		groupPublicKey:     groupPublicKey,    // groupPublicKey is the group public key corresponding to the group secret key
		publicKeyShares:    sharePublicKeys,   // sharePublicKeys are the public key shares corresponding to the private key shares
	}, nil
}

// SignShare generates a signature share using the current private key share.
//
// The function does not add the share to the internal pool of shares and do
// not update the internal state.
// This function is thread safe
func (s *thresholdSigner) SignShare() (Signature, error) {

	share, err := s.currentPrivateKey.Sign(s.message, s.hashAlgo)
	if err != nil {
		if IsInvalidInputsError(err) {
			newInvalidInputsError("share signing failed: %s", err)
		}
		return nil, fmt.Errorf("share signing failed: %w", err)
	}
	return share, nil
}

// retruns and error if given index is valid and nil otherwise
// This function is thread safe
func (s *thresholdSigner) validIndex(orig index) error {
	if int(orig) >= s.size || orig < 0 {
		return newInvalidInputsError(
			"origin input is invalid, should be positive less than %d, got %d",
			s.size, orig)
	}
	return nil
}

// VerifyShare verifies the input signature against the stored message and stored
// key at the input index.
//
// This function does not update the internal state.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - other error if the execution failed
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is thread-safe.
func (s *thresholdSigner) VerifyShare(orig int, share Signature) (bool, error) {
	// validate index
	if err := s.validIndex(index(orig)); err != nil {
		return false, err
	}

	return s.publicKeyShares[orig].Verify(share, s.message, s.hashAlgo)
}

// VerifyThresholdSignature verifies the input signature against the stored
// message and stored group public key.
//
// This function does not update the internal state.
// The function errors if the execution failed.
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is thread-safe.
func (s *thresholdSigner) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	return s.groupPublicKey.Verify(thresholdSignature, s.message, s.hashAlgo)
}

// EnoughShares checks whether there are enough shares to reconstruct a signature.
// The funstion returns true if and only if the number of shares have reached (threshold+1)
// shares.
//
// This function is thread safe
func (s *thresholdSigner) EnoughShares() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.enoughShares()
}

// non thread safe version of EnoughShares
func (s *thresholdSigner) enoughShares() bool {
	// len(s.signers) is always <= s.threshold + 1
	return len(s.shares) == (s.threshold + 1)
}

// HasShare checks whether the internal map contains the share of the given index.
// This function is thread safe
func (s *thresholdSigner) HasShare(orig int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.hasShare(index(orig))
}

// non thread safe version of HasShare
func (s *thresholdSigner) hasShare(orig index) bool {
	_, ok := s.shares[orig]
	return ok
}

// TrustedAdd adds a signature share to the internal pool of shares
// without verifying the signature against the message and the participant's
// public key.
//
// The share is only added if the signer index is valid and has not been
// added yet. Moreover, the share is added only if not enough shares were collected.
// The function returns:
//  - (true, nil) if enough signature shares were already collected and no error occured
//  - (false, nil) if not enough shares were collected and no error occured
//  - (false, error) if index is invalid (InvalidInputsError) or already added (other error)
func (s *thresholdSigner) TrustedAdd(orig int, share Signature) (bool, error) {

	// validate index
	if err := s.validIndex(index(orig)); err != nil {
		return false, err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.hasShare(index(orig)) {
		return false, fmt.Errorf("share for %d was already added", orig)
	}

	enough := s.enoughShares()
	if !enough {
		s.shares[index(orig)] = share
	}
	return s.enoughShares(), nil
}

// VerifyAndAdd verifies a signature share (look at `VerifyShare`),
// and may or may not add the share to the local pool of shares.
//
// The share is only added if the signature is valid, the signer index is valid and has not been
// added yet. Moreover, the share is added only if not enough shares were collected.
// Thee function returns 3 outputs:
//  - First boolean output is true if the share is valid and no error is returned, and false otherwise.
//  - Second boolean output is true if enough shares were collected and no error is returned, and false otherwise.
//  - error is IsInvalidInputsError if input index is invalid, and a random error if an exception occured.
//    (an invalid signature is not considered an invalid input, look at `VerifyShare` for details)
// This function is thread safe
func (s *thresholdSigner) VerifyAndAdd(orig int, share Signature) (bool, bool, error) {

	// validate index
	if err := s.validIndex(index(orig)); err != nil {
		return false, false, err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// check share is new
	if s.hasShare(index(orig)) {
		return false, false, fmt.Errorf("share for %d was already added", orig)
	}

	// verify the share
	verif, err := s.publicKeyShares[index(orig)].Verify(share, s.message, s.hashAlgo)
	if err != nil {
		if IsInvalidInputsError(err) {
			return false, false, newInvalidInputsError(
				"verification of share failed: %v", err)
		}
		return false, false, fmt.Errorf("verification of share failed: %w", err)
	}

	enough := s.enoughShares()
	if verif && !enough {
		s.shares[index(orig)] = share
	}
	return verif, s.enoughShares(), nil
}

// ThresholdSignature returns the threshold signature if the threshold was reached.
// The threshold signature is reconstructed if this was not done in a previous call.
//
// In the case of reconstructing the threshold signature, the function errors
// if not enough shares were collected and if any signature fails the deserialization.
// It also performs a final verification against the stored message and group public key
// and errors if the result is not valid. This is required for the function safety since
// `TrustedAdd` allows adding invalid signatures.
// The function is thread-safe.
func (s *thresholdSigner) ThresholdSignature() (Signature, error) {
	// check cached thresholdSignature
	if s.thresholdSignature != nil {
		return s.thresholdSignature, nil
	}
	// reconstruct the threshold signature
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.enoughShares() {
		thresholdSignature, err := s.reconstructThresholdSignature()
		if err != nil {
			return nil, err
		}
		s.thresholdSignature = thresholdSignature
		return thresholdSignature, nil
	}
	return nil, errors.New("there are not enough signatures shares")
}

// reconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *thresholdSigner) reconstructThresholdSignature() (Signature, error) {
	// sanity check
	if len(s.shares) != s.threshold+1 {
		return nil, errors.New("The number of signature shares is not matching the number of signers")
	}
	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)

	// prepare the C layer inputs
	shares := make([]byte, 0, len(s.shares)*signatureLengthBLSBLS12381)
	signers := make([]index, 0, len(s.shares))
	for index, share := range s.shares {
		shares = append(shares, share...)
		signers = append(signers, index)
	}

	// Lagrange Interpolate at point 0
	result := C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&shares[0]),
		(*C.uint8_t)(&signers[0]), (C.int)(s.threshold+1))

	if result != valid {
		if result == invalid {
			return nil, newInvalidInputsError("a signature share is not a valid BLS signature")
		}
		return nil, errors.New("reading signatures has failed")
	}

	// Verify the computed signature
	verif, err := s.VerifyThresholdSignature(thresholdSignature)
	if err != nil {
		if IsInvalidInputsError(err) {
			return nil, newInvalidInputsError("verify threshold signature failed: %s", err)
		}
		return nil, fmt.Errorf("verify threshold signature failed: %w", err)
	}
	if !verif {
		return nil, errors.New(
			"constructed threshold signature does not verify against the group public key, check shares and public key")
	}

	return thresholdSignature, nil
}

// ReconstructThresholdSignature is a stateless api that takes a list of
// signatures and their signers's indices and returns the threshold signature.
//
// size is the number of participants, it must be in the range [ThresholdSignMinSize..ThresholdSignMaxSize].
// threshold is the threshold value, it must be in the range [MinimumThreshold..size-1].
// The function does not check the validity of the shares, and does not check
// the validity of the resulting signature.
// ReconstructThresholdSignature returns:
// - error if the inputs are not in the correct range, if the threshold is not reached,
//    or if input signers are not distinct.
// - Signature: the threshold signature if there is no returned error, nil otherwise
// If the number of shares reaches the required threshold, only the first threshold+1 shares
// are considered to reconstruct the signature.
func ReconstructThresholdSignature(size int, threshold int,
	shares []Signature, signers []int) (Signature, error) {
	// set BLS settings
	blsInstance.reInit()

	if size < ThresholdSignMinSize || size > ThresholdSignMaxSize {
		return nil, newInvalidInputsError(
			"size should be between %d and %d",
			ThresholdSignMinSize,
			ThresholdSignMaxSize)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, newInvalidInputsError(
			"the threshold must be between %d and %d, got %d",
			MinimumThreshold, size-1,
			threshold)
	}

	if len(shares) != len(signers) {
		return nil, newInvalidInputsError(
			"the number of signature shares is not matching the number of signers")
	}

	if len(shares) < threshold+1 {
		return nil, newInvalidInputsError(
			"the number of signatures does not reach the threshold")
	}

	// map to check signers are distinct
	m := make(map[index]bool)

	// flatten the shares (required by the C layer)
	flatShares := make([]byte, 0, signatureLengthBLSBLS12381*(threshold+1))
	indexSigners := make([]index, 0, threshold+1)
	for i, share := range shares {
		flatShares = append(flatShares, share...)
		// check the index is valid
		if signers[i] >= size || signers[i] < 0 {
			return nil, newInvalidInputsError(
				"signer index #%d is invalid", i)
		}
		// check the index is new
		if _, isSeen := m[index(signers[i])]; isSeen {
			return nil, newInvalidInputsError(
				"%d is a duplicate signer", index(signers[i]))
		}
		m[index(signers[i])] = true
		indexSigners = append(indexSigners, index(signers[i]))
	}

	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	if C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&flatShares[0]),
		(*C.uint8_t)(&indexSigners[0]), (C.int)(threshold+1),
	) != valid {
		return nil, newInvalidInputsError("reading signatures has failed")
	}
	return thresholdSignature, nil
}

// EnoughShares is a stateless function that takes the value of the threshold
// and a shares number and returns true if the shares number is enough
// to reconstruct a threshold signature.
func EnoughShares(threshold int, sharesNumber int) (bool, error) {
	if threshold < MinimumThreshold {
		return false, newInvalidInputsError(
			"the threshold can't be smaller than %d, got %d",
			MinimumThreshold, threshold)
	}
	return sharesNumber > threshold, nil
}

// ThresholdKeyGen is a key generation for a BLS-based
// threshold signature scheme with a trusted dealer.
func ThresholdKeyGen(size int, threshold int, seed []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	if size < ThresholdSignMinSize || size > ThresholdSignMaxSize {
		return nil, nil, nil, newInvalidInputsError(
			"size should be between %d and %d, got %d",
			ThresholdSignMinSize,
			ThresholdSignMaxSize,
			size)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, nil, nil, newInvalidInputsError(
			"the threshold must be between %d and %d, got %d",
			MinimumThreshold,
			size-1,
			threshold)
	}

	// set BLS settings
	blsInstance.reInit()

	// the scalars x and G2 points y
	x := make([]scalar, size)
	y := make([]pointG2, size)
	var X0 pointG2

	// seed relic
	if err := seedRelic(seed); err != nil {
		if IsInvalidInputsError(err) {
			return nil, nil, nil, newInvalidInputsError(
				"seeding relic failed: %s",
				err)
		}
		return nil, nil, nil, fmt.Errorf("seeding relic failed: %w", err)
	}
	// Generate a polynomial P in Zr[X] of degree t
	a := make([]scalar, threshold+1)
	randZrStar(&a[0])
	for i := 1; i < threshold+1; i++ {
		randZr(&a[i])
	}
	// compute the shares
	for i := index(1); int(i) <= size; i++ {
		C.Zr_polynomialImage(
			(*C.bn_st)(&x[i-1]),
			(*C.ep2_st)(&y[i-1]),
			(*C.bn_st)(&a[0]), (C.int)(len(a)),
			(C.uint8_t)(i),
		)
	}
	// group public key
	genScalarMultG2(&X0, &a[0])
	// export the keys
	skShares := make([]PrivateKey, size)
	pkShares := make([]PublicKey, size)
	var pkGroup PublicKey
	for i := 0; i < size; i++ {
		skShares[i] = newPrKeyBLSBLS12381(&x[i])
		pkShares[i] = newPubKeyBLSBLS12381(&y[i])
	}
	pkGroup = newPubKeyBLSBLS12381(&X0)
	return skShares, pkShares, pkGroup, nil
}
