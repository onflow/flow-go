// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "bls_thresholdsign_include.h"
import "C"

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
)

// A threshold signature scheme allows any subset of (t+1)
// valid signature shares to reconstruct the threshold signature.
// Up to (t) shares do not reveal any information about the threshold
// signature.
// Although the API allows using arbitrary values of (t),
// the threshold signature scheme is secure in the presence of up to (t)
// malicious participants when (t < n/2).
// In order to optimize equally for unforgeability and robustness,
// the input threshold value (t) should be set to t = floor((n-1)/2).

// ThresholdSignatureInspector is an inspector of the threshold signature protocol.
// The interface only allows inspecting the threshold signing protocol without taking part in it.
type ThresholdSignatureInspector interface {
	// VerifyShare verifies the input signature against the stored message and stored
	// key at the input index. This function does not update the internal state.
	// The function is thread-safe.
	// Returns:
	//  - (true, nil) if the signature is valid
	//  - (false, nil) if `orig` is a valid index but the signature share is invalid
	//  - (false, InvalidInputsError) if `orig` is an invalid index value
	//  - (false, error) for all other unexpected errors
	VerifyShare(orig int, share Signature) (bool, error)

	// VerifyThresholdSignature verifies the input signature against the stored
	// message and stored group public key. It does not update the internal state.
	// The function is thread-safe.
	// Returns:
	//  - (true, nil) if the signature is valid
	//  - (false, nil) if the signature is invalid
	//  - (false, error) for all other unexpected errors
	VerifyThresholdSignature(thresholdSignature Signature) (bool, error)

	// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
	// a group signature. This function is thread safe and locks the internal state.
	// Returns:
	//  - true if and only if at least (threshold+1) shares were added
	EnoughShares() bool

	// TrustedAdd adds a signature share to the internal pool of shares
	// without verifying the signature against the message and the participant's
	// public key. This function is thread safe and locks the internal state.
	//
	// The share is only added if the signer index is valid and has not been
	// added yet. Moreover, the share is added only if not enough shares were collected.
	// The function returns:
	//  - (true, nil) if enough signature shares were already collected and no error occured
	//  - (false, nil) if not enough shares were collected and no error occured
	//  - (false, InvalidInputsError) if index is invalid
	//  - (false, duplicatedSignerError) if a signature for the index was previously added
	TrustedAdd(orig int, share Signature) (bool, error)

	// VerifyAndAdd verifies a signature share (same as `VerifyShare`),
	// and may or may not add the share to the local pool of shares.
	// This function is thread safe and locks the internal state.
	//
	// The share is only added if the signature is valid, the signer index is valid and has not been
	// added yet. Moreover, the share is added only if not enough shares were collected.
	// The function returns 3 outputs:
	//  - First boolean output is true if the share is valid and no error is returned, and false otherwise.
	//  - Second boolean output is true if enough shares were collected and no error is returned, and false otherwise.
	//  - error is InvalidInputsError if input index is invalid, duplicatedSignerError if signer was added,
	//	   and a random error if an exception occurred.
	//    (an invalid signature is not considered an invalid input, look at `VerifyShare` for details)
	VerifyAndAdd(orig int, share Signature) (bool, bool, error)

	// HasShare checks whether the internal map contains the share of the given index.
	// This function is thread safe.
	// The function errors with InvalidInputsError if the index is invalid.
	HasShare(orig int) (bool, error)

	// ThresholdSignature returns the threshold signature if the threshold was reached.
	// The threshold signature is reconstructed only once and is cached for subsequent calls.
	//
	// Returns:
	// - (signature, nil) if no error occured
	// - (nil, notEnoughSharesError) if not enough shares were collected
	// - (nil, invalidInputsError) if at least one collected share does not serialize to a valid BLS signature,
	//    or if the constructed signature failed to verify against the group public key and stored message. This post-verification
	//    is required  for safety, as `TrustedAdd` allows adding invalid signatures.
	// - (nil, error) for any other unexpected error.
	ThresholdSignature() (Signature, error)
}

// ThresholdSignatureParticipant is a participant in a threshold signature protocol.
// A participant is able to participate in a threshold signing protocol as well as inspecting the
// protocol.
type ThresholdSignatureParticipant interface {
	ThresholdSignatureInspector
	// SignShare generates a signature share using the current private key share.
	//
	// The function does not add the share to the internal pool of shares and do
	// not update the internal state.
	// This function is thread safe
	// No error is expected unless an unexpected exception occurs
	SignShare() (Signature, error)
}

// duplicatedSignerError is an error returned when TrustedAdd or VerifyAndAdd encounter
// a signature share that has been already added to the internal state.
type duplicatedSignerError struct {
	error
}

// duplicatedSignerErrorf constructs a new duplicatedSignerError
func duplicatedSignerErrorf(msg string, args ...interface{}) error {
	return &duplicatedSignerError{error: fmt.Errorf(msg, args...)}
}

// IsDuplicatedSignerError checks if the input error is a duplicatedSignerError
func IsDuplicatedSignerError(err error) bool {
	var target *duplicatedSignerError
	return errors.As(err, &target)
}

// notEnoughSharesError is an error returned when ThresholdSignature is called
// and not enough shares have been collected.
type notEnoughSharesError struct {
	error
}

// notEnoughSharesErrorf constructs a new notEnoughSharesError
func notEnoughSharesErrorf(msg string, args ...interface{}) error {
	return &notEnoughSharesError{error: fmt.Errorf(msg, args...)}
}

// IsNotEnoughSharesError checks if the input error is a notEnoughSharesError
func IsNotEnoughSharesError(err error) bool {
	var target *notEnoughSharesError
	return errors.As(err, &target)
}

// TODO: TO DELETE IN V2
// ONLY HERE TO ALLOW V1 build and test

type thresholdSigner struct {
	// size of the group
	size int
	// the thresold t of the scheme where (t+1) shares are
	// required to reconstruct a signature
	threshold int
	// the index of the current participant
	myIndex int
	// the current participant private key (a DKG output)
	myPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []PublicKey
	// the hasher to be used for all signatures
	hashAlgo hash.Hasher
	// the message to be signed. Siganture shares and the threshold signature
	// are verified using this message
	messageToSign []byte
	// the valid signature shares received from other participants
	shares []byte // simulates an array of Signatures
	// (or a matrix of by bytes) to accommodate a cgo constraint
	// the list of signers corresponding to the list of shares
	signers []index
	// the threshold signature. It is equal to nil if less than (t+1) shares are
	// received
	thresholdSignature Signature
	// stagedShare stores a temporary share that is staged but not committed yet
	// to the list of shares. stagedOrig is the index related to stagedShare
	stagedShare Signature
	stagedOrig  int
}

// NewThresholdSigner creates a new instance of Threshold signer using BLS.
// hash is the hashing algorithm to be used.
// size is the number of participants, it must be in the range [ThresholdSignMinSize..ThresholdSignMaxSize]
// threshold is the threshold value, it must be in the range [MinimumThreshold..size-1]
func NewThresholdSigner(size int, threshold int, myIndex int, hashAlgo hash.Hasher) (*thresholdSigner, error) {
	if size < ThresholdSignMinSize || size > ThresholdSignMaxSize {
		return nil, invalidInputsErrorf(
			"size should be between %d and %d, got %d",
			ThresholdSignMinSize, ThresholdSignMaxSize, size)
	}
	if myIndex >= size || myIndex < 0 {
		return nil, invalidInputsErrorf(
			"The current index must be between 0 and %d, got %d",
			size-1, myIndex)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, invalidInputsErrorf(
			"The threshold must be between %d and %d, got %d",
			MinimumThreshold, size-1, threshold)
	}

	// set BLS settings
	blsInstance.reInit()

	// internal list of valid signature shares
	shares := make([]byte, 0, (threshold+1)*SignatureLenBLSBLS12381)
	signers := make([]index, 0, threshold+1)

	return &thresholdSigner{
		size:               size,
		threshold:          threshold,
		myIndex:            myIndex,
		hashAlgo:           hashAlgo,
		shares:             shares,
		signers:            signers,
		thresholdSignature: nil,
		stagedShare:        nil,
		stagedOrig:         -1,
	}, nil
}

// SetKeys sets the private and public keys needed by the threshold signature
// - groupPublicKey is the group public key corresponding to the group secret key
// - sharePublicKeys are the public key shares corresponding to the participants private
// key shares.
// - myPrivateKey is the current participant's own private key share
// Output keys of ThresholdSignKeyGen or DKG protocols could be used as the input
// keys to this function.
func (s *thresholdSigner) SetKeys(myPrivateKey PrivateKey,
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey) error {

	if len(sharePublicKeys) != s.size {
		return invalidInputsErrorf(
			"size of public key shares should be %d, but got %d",
			s.size, len(sharePublicKeys))
	}

	// clear existing shares signed with previous keys.
	s.ClearShares()

	s.myPrivateKey = myPrivateKey
	s.groupPublicKey = groupPublicKey
	s.publicKeyShares = sharePublicKeys
	return nil
}

// SetMessageToSign sets the next message to be signed.
// All signatures shares of a different message are ignored
func (s *thresholdSigner) SetMessageToSign(message []byte) {
	s.ClearShares()
	s.messageToSign = message
}

// SignShare generates a signature share using the current private key share
func (s *thresholdSigner) SignShare() (Signature, error) {
	if s.myPrivateKey == nil {
		return nil, errors.New("the private key of the current participant is not set")
	}
	// sign
	share, err := s.myPrivateKey.Sign(s.messageToSign, s.hashAlgo)
	if err != nil {
		if IsInvalidInputsError(err) {
			invalidInputsErrorf("share signature failed: %s", err)
		}
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	// add the participant own signature
	valid, err := s.AddShare(s.myIndex, share)
	if err != nil {
		if IsInvalidInputsError(err) {
			return nil, invalidInputsErrorf("share signature failed: %s", err)
		}
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	if !valid {
		return nil, errors.New("the current participant private and public keys do not match")
	}
	return share, nil
}

// verifyShare verifies a signature share using the signer's public key
func (s *thresholdSigner) verifyShare(share Signature, signerIndex index) (bool, error) {
	if len(s.publicKeyShares) != s.size {
		return false, errors.New("the participant public keys are not set")
	}

	return s.publicKeyShares[signerIndex].Verify(share, s.messageToSign, s.hashAlgo)
}

// VerifyThresholdSignature verifies a threshold signature using the group public key
func (s *thresholdSigner) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	if s.groupPublicKey == nil {
		return false, errors.New("the group public key is not set")
	}
	return s.groupPublicKey.Verify(thresholdSignature, s.messageToSign, s.hashAlgo)
}

// ClearShares clears the shares and signers lists
func (s *thresholdSigner) ClearShares() {
	s.thresholdSignature = nil
	s.emptyStagedShare()
	s.signers = s.signers[:0]
	s.shares = s.shares[:0]
}

// EnoughShares checks whether there are enough shares to reconstruct a signature
func (s *thresholdSigner) EnoughShares() bool {
	// note: len(s.signers) is always <= s.threshold + 1
	return len(s.signers) == (s.threshold + 1)
}

func (s *thresholdSigner) emptyStagedShare() {
	s.stagedShare = nil
	s.stagedOrig = -1
}

// AddShare adds a signature share to the local list of shares.
//
// If the share is valid, not perviously added and the threshold is not reached yet,
// it is appended to the local list of valid shares. This is equivelent to staging
// and commiting a share at the same step.
// The function returns true if the share is valid and new, false otherwise.
func (s *thresholdSigner) AddShare(orig int, share Signature) (bool, error) {
	// stage the share
	verif, err := s.VerifyAndStageShare(orig, share)
	if err != nil {
		if IsInvalidInputsError(err) {
			return false, invalidInputsErrorf("add signature failed: %s", err)
		}
		return false, fmt.Errorf("add signature failed: %w", err)
	}
	if verif {
		// commit the share
		err = s.CommitShare()
		if err != nil {
			if IsInvalidInputsError(err) {
				return true, invalidInputsErrorf("add signature failed: %s", err)
			}
			return true, fmt.Errorf("add signature failed: %w", err)
		}
	}
	return verif, nil
}

// VerifyAndStageShare verifies a signature share without adding it to the local list of shares.
//
// If the share is valid and new, it is stored temporarily till it is committed to the shares
// list using CommitShare.
// Any call to VerifyAndStageShare, AddShare or CommitShare empties the staged share.
// If the threshold is already reached, the VerifyAndStageShare still verifies the share.
// The function returns true if the share is valid and new, false otherwise.
func (s *thresholdSigner) VerifyAndStageShare(orig int, share Signature) (bool, error) {
	// empty the staged share
	s.emptyStagedShare()

	if orig >= s.size || orig < 0 {
		return false, invalidInputsErrorf(
			"origin input is invalid, should be positive less than %d, got %d",
			s.size, orig)
	}

	verif, err := s.verifyShare(share, index(orig))
	if err != nil {
		if IsInvalidInputsError(err) {
			return false, invalidInputsErrorf(
				"verification of share failed: %s", err)
		}
		return false, fmt.Errorf("verification of share failed: %w", err)
	}

	// check if the share is new
	isSeen := false
	for _, e := range s.signers {
		if e == index(orig) {
			isSeen = true
			break
		}
	}

	// assign the temp share
	if verif && !isSeen {
		// store the share temporarily
		s.stagedShare = share
		s.stagedOrig = orig
		return true, nil
	}
	return false, nil
}

// CommitShare commits the staged signature to the local list of shares if the threshold
// is not reached.
//
// The staged share is stored by the latest call to VerifyAndStageShare. Calling CommitShare
// empties the staged share.
func (s *thresholdSigner) CommitShare() error {
	if s.stagedShare == nil || s.stagedOrig == -1 {
		return fmt.Errorf("there is no signature to commit")
	}
	// append the staged share
	if !s.EnoughShares() {
		s.shares = append(s.shares, s.stagedShare...)
		s.signers = append(s.signers, index(s.stagedOrig))
	}

	// empty the staged share
	s.emptyStagedShare()
	return nil
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
	return nil, errors.New("there are not enough signatures shares")
}

// ReconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *thresholdSigner) reconstructThresholdSignature() (Signature, error) {
	// sanity check
	if len(s.shares) != len(s.signers)*signatureLengthBLSBLS12381 {
		s.ClearShares()
		return nil, invalidInputsErrorf("The number of signature shares is not matching the number of signers")
	}
	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	result := C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&s.shares[0]),
		(*C.uint8_t)(&s.signers[0]), (C.int)(len(s.signers)))
	if result != valid {
		if result == invalid { // sanity check, but shouldn't happen
			return nil, invalidInputsErrorf("a signature share is not valid")
		}
		return nil, errors.New("reading signatures has failed")
	}

	// Verify the computed signature
	verif, err := s.VerifyThresholdSignature(thresholdSignature)
	if err != nil {
		if IsInvalidInputsError(err) {
			return nil, invalidInputsErrorf("verify threshold signature failed: %s", err)
		}
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
		return nil, invalidInputsErrorf(
			"size should be between %d and %d",
			ThresholdSignMinSize,
			ThresholdSignMaxSize)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, invalidInputsErrorf(
			"The threshold must be between %d and %d, got %d",
			MinimumThreshold, size-1,
			threshold)
	}

	if len(shares) != len(signers) {
		return nil, invalidInputsErrorf(
			"The number of signature shares is not matching the number of signers")
	}

	if len(shares) < threshold+1 {
		return nil, invalidInputsErrorf(
			"The number of signatures does not reach the threshold")
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
			return nil, invalidInputsErrorf(
				"signer index #%d is invalid", i)
		}
		// check the index is new
		if _, isSeen := m[index(signers[i])]; isSeen {
			return nil, invalidInputsErrorf(
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
		return nil, errors.New("reading signatures has failed")
	}
	return thresholdSignature, nil
}

// ThresholdSignKeyGen is a key generation for a BLS-based
// threshold signature scheme with a trusted dealer.
func ThresholdSignKeyGen(size int, threshold int, seed []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	if size < ThresholdSignMinSize || size > ThresholdSignMaxSize {
		return nil, nil, nil, invalidInputsErrorf(
			"size should be between %d and %d, got %d",
			ThresholdSignMinSize,
			ThresholdSignMaxSize,
			size)
	}
	if threshold >= size || threshold < MinimumThreshold {
		return nil, nil, nil, invalidInputsErrorf(
			"The threshold must be between %d and %d, got %d",
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
			return nil, nil, nil, invalidInputsErrorf(
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
