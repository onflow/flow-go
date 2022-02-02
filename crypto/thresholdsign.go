package crypto

import "C"

import (
	"errors"
	"fmt"
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
	//  - (true, nil) if enough signature shares were already collected and no error occurred
	//  - (false, nil) if not enough shares were collected and no error occurred
	//  - (false, InvalidInputsError) if index is invalid
	//  - (false, duplicatedSignerError) if a signature for the index was previously added
	TrustedAdd(orig int, share Signature) (bool, error)

	// VerifyAndAdd verifies a signature share (same as `VerifyShare`),
	// and may or may not add the share to the local pool of shares.
	// This function is thread safe and locks the internal state.
	//
	// The share is only added if the signature is valid, the signer index is valid and has not been
	// added yet. Moreover, the share is added only if not enough shares were collected.
	// Boolean returns:
	//  - First boolean output is true if the share is valid and no error is returned, and false otherwise.
	//  - Second boolean output is true if enough shares were collected and no error is returned, and false otherwise.
	// Error returns:
	//  - invalidInputsError if input index is invalid. A signature that doesn't verify against the signer's
	//    public key is not considered an invalid input.
	//  - duplicatedSignerError if signer was already added.
	//  - other errors if an unexpected exception occurred.
	VerifyAndAdd(orig int, share Signature) (bool, bool, error)

	// HasShare checks whether the internal map contains the share of the given index.
	// This function is thread safe.
	// The function errors with InvalidInputsError if the index is invalid.
	HasShare(orig int) (bool, error)

	// ThresholdSignature returns the threshold signature if the threshold was reached.
	// The threshold signature is reconstructed only once and is cached for subsequent calls.
	//
	// Returns:
	// - (signature, nil) if no error occurred
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
