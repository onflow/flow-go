// +build relic

package signature

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/module"
)

// SignatureAggregatorSameMessage aggregates BLS signatures of the same message from different signers.
// The public keys and message are aggreed upon upfront.
//
// The module does not support signatures with multiplicity higher than 1. Each signer is allowed
// to sign at most once.
//
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
//
// Implementation of SignatureAggregator is not thread-safe, the caller should
// make sure the calls are concurrent safe.
type SignatureAggregatorSameMessage struct {
	// TODO: initial incomplete fields that will evolve
	message          []byte
	hasher           hash.Hasher
	n                int                // number of participants indexed from 0 to n-1
	publicKeys       []crypto.PublicKey // keys indexed from 0 to n-1, signer i is assigned to public key i
	indexToSignature map[int]string     // signatures indexed by the signer index

	// the below items are related to public keys aggregation and the greedy aggrgetion algorithm
	lastSigners       map[int]struct{} // maps the signers in the latest call to aggregate keys
	lastAggregatedKey crypto.PublicKey // the latest aggregated public key
	sync.RWMutex                       // the above "latest" data only make sense in a concurrent safe model, the lock maintains the thread-safety.
	// since the caller should not be aware of the internal non thread-safe algorithm.
}

// NewSignatureAggregatorSameMessage returns a new SignatureAggregatorSameMessage structure.
//
// A new SignatureAggregatorSameMessage is needed for each set of public keys. If the key set changes,
// a new structure needs to be instantiated.
// The number of public keys fixes the number of partcipantn. Each participant i is defined
// by public key publicKeys[i]
// The function errors with ErrInvalidInputs if any input is invalid.
func NewSignatureAggregatorSameMessage(
	message []byte, // message to be aggregate signatures for
	dsTag string, // domain separation tag used for signatures
	publicKeys []crypto.PublicKey, // public keys of participants agreed upon upfront
) (*SignatureAggregatorSameMessage, error) {

	if len(publicKeys) == 0 {
		return nil, newErrInvalidInputs("number of participants must be larger than 0, got %d", len(publicKeys))
	}
	// sanity check for BLS keys
	for i, key := range publicKeys {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, newErrInvalidInputs("key at index %d is not a BLS key", i)
		}
	}

	return &SignatureAggregatorSameMessage{
		message:          message,
		hasher:           crypto.NewBLSKMAC(dsTag),
		n:                len(publicKeys),
		publicKeys:       publicKeys,
		indexToSignature: make(map[int]string),

		lastSigners:       make(map[int]struct{}),
		lastAggregatedKey: crypto.NeutralBLSPublicKey(),
		RWMutex:           sync.RWMutex{},
	}, nil
}

// Verify verifies the input signature under the stored message and stored
// key at the input index.
//
// This function does not update the internal state.
// The function errors:
//  - ErrInvalidInputs if the index input is invalid
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) Verify(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, newErrInvalidInputs("input index %d is invalid", signer)
	}
	return s.publicKeys[signer].Verify(sig, s.message, s.hasher)
}

// VerifyAndAdd verifies the input signature under the stored message and stored
// key at the input index. If the verification passes, the signature is added to the internal
// signature state.
// The function errors:
//  - ErrInvalidInputs if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) VerifyAndAdd(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, newErrInvalidInputs("input index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return false, newErrDuplicatedSigner("signer %d was already added", signer)
	}
	// signature is new
	ok, err := s.publicKeys[signer].Verify(sig, s.message, s.hasher)
	if ok {
		s.indexToSignature[signer] = string(sig)
	}
	return ok, err
}

// TrustedAdd adds a signature to the internal state without verifying it.
//
// The Aggregate function makes a sanity check on the aggregated signature and only
// outputs valid signatures. This would detect if TrustedAdd has added any invalid
// signature.
// The function errors:
//  - ErrInvalidInputs if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) TrustedAdd(signer int, sig crypto.Signature) error {
	if signer >= s.n || signer < 0 {
		return newErrInvalidInputs("input index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return newErrDuplicatedSigner("signer %d was already added", signer)
	}
	// signature is new
	s.indexToSignature[signer] = string(sig)
	return nil
}

// Aggregate aggregates the stored BLS signatures and returns the aggregated signature.
//
// Aggregate attempts to aggregate the internal signatures and returns the resulting signature.
// The function performs a final verification and errors if any signature fails the desrialization
// or if the aggregated signature is not valid.
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
// The function is not thread-safe.
//
// TODO : When compacting the list of signers, update the return from []int
// to a compact bit vector.
func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	sharesNum := len(s.indexToSignature)
	signatures := make([]crypto.Signature, 0, sharesNum)
	indices := make([]int, 0, sharesNum)
	for index, sig := range s.indexToSignature {
		signatures = append(signatures, []byte(sig))
		indices = append(indices, index)
	}

	aggregatedSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		return nil, nil, errors.New("BLS signature aggregtion failed")
	}
	ok, err := s.VerifyAggregation(indices, aggregatedSignature)
	if err != nil {
		return nil, nil, errors.New("BLS signature verification failed")
	}
	if !ok {
		return nil, nil, errors.New("resulting BLS aggregated signatutre is invalid")
	}
	return indices, aggregatedSignature, nil
}

// VerifyAggregation verifies an aggregated signature against the stored message and the stored
// keys corresponding to the input signers.
// Aggregating the keys of the signers internally is optimized to only look at the keys delta
// compared to the latest execution of the function. The function is therefore not thread-safe.
// The function errors:
//  - ErrInvalidInputs if the indices are invalid
//  - random error if the execution failed
func (s *SignatureAggregatorSameMessage) VerifyAggregation(signers []int, sig crypto.Signature) (bool, error) {
	panic("implement me")
}

// SetMessage sets a new message in the structure.
// This allows re-using the same structure with the same keys for different messages.
// Once called, the fuction empties all internally stored signatures.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) SetMessage(message []byte) { // we could also update the DST here
	s.message = message
	// empty existing signatures
	s.indexToSignature = make(map[int]string)
}

// aggregatedKey returns the aggregated public key of the input signers.
func (s *SignatureAggregatorSameMessage) aggregatedKey(signers []int) (crypto.PublicKey, error) {

	// this greedy algorithm assumes the signers set does not vary much from one call
	// to aggregatedKey to another. It computes the delta of signers compared to the
	// latest list of signers and adjust the latest aggregated public key. This is faster
	// than aggregating the public keys from scratch at each call.

	// read lock to read consistent last key and last signers
	s.RLock()
	// get the signers delta and update the last list for the next comparison
	newSignerKeys, missingSignerKeys, updatedSignerSet := s.deltaKeys(signers)
	lastKey := s.lastAggregatedKey
	s.RUnlock()

	// add the new keys
	var err error
	updatedKey, err := crypto.AggregateBLSPublicKeys(append(newSignerKeys, lastKey))
	if err != nil {
		return nil, fmt.Errorf("adding new staking keys failed: %w", err)
	}
	// remove the missing keys
	updatedKey, err = crypto.RemoveBLSPublicKeys(updatedKey, missingSignerKeys)
	if err != nil {
		return nil, fmt.Errorf("removing missing staking keys failed: %w", err)
	}

	// update the latest list and public key.
	s.Lock()
	s.lastSigners = updatedSignerSet
	s.lastAggregatedKey = updatedKey
	s.Unlock()
	return updatedKey, nil
}

// keysDelta computes the delta between the reference s.lastSigners
// and the input identity list.
// It returns a list of the new signer keys, a list of the missing signer keys and the new map of signers.
func (s *SignatureAggregatorSameMessage) deltaKeys(signers []int) (
	[]crypto.PublicKey, []crypto.PublicKey, map[int]struct{}) {

	var newSignerKeys, missingSignerKeys []crypto.PublicKey

	// create a map of the input list,
	// and check the new signers
	signersMap := make(map[int]struct{})
	for _, signer := range signers {
		signersMap[signer] = struct{}{}
		_, ok := s.lastSigners[signer]
		if !ok {
			newSignerKeys = append(newSignerKeys, s.publicKeys[signer])
		}
	}

	// look for missing signers
	for signer, _ := range s.lastSigners {
		_, ok := signersMap[signer]
		if !ok {
			missingSignerKeys = append(missingSignerKeys, s.publicKeys[signer])
		}
	}
	return newSignerKeys, missingSignerKeys, signersMap
}

//------------------------------------------

// TODO : to delete in V2
// AggregationVerifier is an aggregating verifier that can verify signatures and
// verify aggregated signatures of the same message.
// *Important: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationVerifier struct {
	hasher hash.Hasher
}

// TODO : to delete in V2
// NewAggregationVerifier creates a new aggregation verifier, which can only
// verify signatures. *Important*: the aggregation verifier can only verify
// signatures in the context of the provided KMAC tag.
func NewAggregationVerifier(tag string) *AggregationVerifier {
	av := &AggregationVerifier{
		hasher: crypto.NewBLSKMAC(tag),
	}
	return av
}

// TODO : to delete in V2
// Verify will verify the given signature against the given message and public key.
func (av *AggregationVerifier) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, av.hasher)
}

// TODO : to delete in V2
// VerifyMany will verify the given aggregated signature against the given message and the
// provided public keys.
func (av *AggregationVerifier) VerifyMany(msg []byte, sig crypto.Signature, keys []crypto.PublicKey) (bool, error) {

	// bls multi-signature verification with a single message
	valid, err := crypto.VerifyBLSSignatureOneMessage(keys, sig, msg, av.hasher)
	if err != nil {
		return false, fmt.Errorf("could not verify aggregated signature: %w", err)
	}

	return valid, nil
}

// TODO : to delete in V2
// AggregationProvider is an aggregating signer and verifier that can create/verify
// signatures, as well as aggregating & verifying aggregated signatures.
// *Important*: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationProvider struct {
	*AggregationVerifier
	local module.Local
}

// TODO : to delete in V2
// NewAggregationProvider creates a new aggregation provider using the given private
// key to generate signatures. *Important*: the aggregation provider can only
// create and verify signatures in the context of the provided HMAC tag.
func NewAggregationProvider(tag string, local module.Local) *AggregationProvider {
	ap := &AggregationProvider{
		AggregationVerifier: NewAggregationVerifier(tag),
		local:               local,
	}
	return ap
}

// TODO : to delete in V2
// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (ap *AggregationProvider) Sign(msg []byte) (crypto.Signature, error) {
	return ap.local.Sign(msg, ap.hasher)
}

// TODO : to delete in V2
// Aggregate will aggregate the given signatures into one aggregated signature.
func (ap *AggregationProvider) Aggregate(sigs []crypto.Signature) (crypto.Signature, error) {

	// BLS aggregation
	sig, err := crypto.AggregateBLSSignatures(sigs)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate BLS signatures: %w", err)
	}
	return sig, nil
}
