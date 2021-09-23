// +build relic

package signature

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
)

// SignatureAggregatorSameMessage aggregates BLS signatures of the same message from different signers.
// The public keys and message are aggreed upon upfront.
//
// Currently, the module does not support signatures with multiplicity higher than 1. Each signer is allowed
// to sign at most once.
//
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
//
// Implementation of SignatureAggregator is not thread-safe, the caller should
// make sure the calls are concurrent safe.
type SignatureAggregatorSameMessage struct {
	message          []byte
	hasher           hash.Hasher
	n                int                // number of participants indexed from 0 to n-1
	publicKeys       []crypto.PublicKey // keys indexed from 0 to n-1, signer i is assigned to public key i
	indexToSignature map[int]string     // signatures indexed by the signer index
	cashedSignature  crypto.Signature   // cached aggrgeted signature
}

// NewSignatureAggregatorSameMessage returns a new SignatureAggregatorSameMessage structure.
//
// A new SignatureAggregatorSameMessage is needed for each set of public keys. If the key set changes,
// a new structure needs to be instantiated. Participants are defined by their public keys, and are
// indexed from 0 to n-1 where n is the length of the public key slice.
// The function errors with engine.InvalidInputError if:
//  - length of keys is zero
//  - any input public key is not a BLS 12-381 key
func NewSignatureAggregatorSameMessage(
	message []byte, // message to be aggregate signatures for
	dsTag string, // domain separation tag used for signatures
	publicKeys []crypto.PublicKey, // public keys of participants agreed upon upfront
) (*SignatureAggregatorSameMessage, error) {

	if len(publicKeys) == 0 {
		return nil, engine.NewInvalidInputErrorf("number of participants must be larger than 0, got %d", len(publicKeys))
	}
	// sanity check for BLS keys
	for i, key := range publicKeys {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, engine.NewInvalidInputErrorf("key at index %d is not a BLS key", i)
		}
	}

	return &SignatureAggregatorSameMessage{
		message:          message,
		hasher:           crypto.NewBLSKMAC(dsTag),
		n:                len(publicKeys),
		publicKeys:       publicKeys,
		indexToSignature: make(map[int]string),
		cashedSignature:  nil,
	}, nil
}

// Verify verifies the input signature under the stored message and stored
// key at the input index.
//
// This function does not update the internal state.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) Verify(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, engine.NewInvalidInputErrorf("input index %d is invalid", signer)
	}
	return s.publicKeys[signer].Verify(sig, s.message, s.hasher)
}

// VerifyAndAdd verifies the input signature under the stored message and stored
// key at the input index. If the verification passes, the signature is added to the internal
// signature state.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) VerifyAndAdd(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, engine.NewInvalidInputErrorf("input index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return false, newErrDuplicatedSigner("signer %d was already added", signer)
	}
	// signature is new
	ok, err := s.publicKeys[signer].Verify(sig, s.message, s.hasher)
	if ok {
		s.cashedSignature = nil
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
//  - engine.InvalidInputErrorf if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) TrustedAdd(signer int, sig crypto.Signature) error {
	if signer >= s.n || signer < 0 {
		return engine.NewInvalidInputErrorf("input index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return newErrDuplicatedSigner("signer %d was already added", signer)
	}
	// signature is new
	s.cashedSignature = nil
	s.indexToSignature[signer] = string(sig)
	return nil
}

// HasSignature checks if a signer has already provided a valid signature.
//
// The function errors:
//  - engine.InvalidInputError if the index input is invalid
//  - random error if the execution failed
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) HasSignature(signer int) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, engine.NewInvalidInputErrorf("input index %d is invalid", signer)
	}
	_, ok := s.indexToSignature[signer]
	return ok, nil
}

// Aggregate aggregates the stored BLS signatures and returns the aggregated signature.
//
// Aggregate attempts to aggregate the internal signatures and returns the resulting signature.
// The function performs a final verification and errors if any signature fails the desrialization
// or if the aggregated signature is not valid. It also errors if no signatures were added.
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
// The function is not thread-safe.
//
// TODO : When compacting the list of signers, update the return from []int
// to a compact bit vector.
func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	sharesNum := len(s.indexToSignature)
	indices := make([]int, 0, sharesNum)
	for index, _ := range s.indexToSignature {
		indices = append(indices, index)
	}

	// check if signature was already computed
	if s.cashedSignature != nil {
		return indices, s.cashedSignature, nil
	}

	signatures := make([]crypto.Signature, 0, sharesNum)
	for _, sig := range s.indexToSignature {
		signatures = append(signatures, []byte(sig))
	}

	aggregatedSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		return nil, nil, fmt.Errorf("BLS signature aggregtion failed %w", err)
	}
	ok, err := s.VerifyAggregate(indices, aggregatedSignature)
	if err != nil {
		return nil, nil, fmt.Errorf("BLS signature aggregtion failed %w", err)
	}
	if !ok {
		return nil, nil, errors.New("resulting BLS aggregated signatutre is invalid")
	}
	s.cashedSignature = aggregatedSignature
	return indices, aggregatedSignature, nil
}

// VerifyAggregate verifies an aggregated signature against the stored message and the stored
// keys corresponding to the input signers.
// Aggregating the keys of the signers internally is optimized to only look at the keys delta
// compared to the latest execution of the function. The function is therefore not thread-safe.
// The function errors:
//  - engine.InvalidInputErrorf if the indices are invalid or the signers list is empty.
//  - random error if the execution failed
func (s *SignatureAggregatorSameMessage) VerifyAggregate(signers []int, sig crypto.Signature) (bool, error) {
	sharesNum := len(signers)
	keys := make([]crypto.PublicKey, 0, sharesNum)
	if sharesNum == 0 {
		return false, engine.NewInvalidInputErrorf("signers list is empty")
	}
	for _, signer := range signers {
		if signer >= s.n || signer < 0 {
			return false, engine.NewInvalidInputErrorf("input index %d is invalid", signer)
		}
		keys = append(keys, s.publicKeys[signer])
	}
	KeyAggregate, err := crypto.AggregateBLSPublicKeys(keys)
	if err != nil {
		return false, fmt.Errorf("aggregating public keys failed: %w", err)
	}
	ok, err := KeyAggregate.Verify(sig, s.message, s.hasher)
	if err != nil {
		return false, fmt.Errorf("signature verification failed: %w", err)
	}
	return ok, nil
}

// PublicKeyAggregator aggregates BLS public keys in an optimized manner.
// It uses a greedy algorithm to compute the aggregated key based on the latest
// computed key and the delta of keys.
// A caller can use a classic stateless aggregation if the optimization is not needed.
//
// The structure is thread safe.
type PublicKeyAggregator struct {
	n                 int                // number of participants indexed from 0 to n-1
	publicKeys        []crypto.PublicKey // keys indexed from 0 to n-1, signer i is assigned to public key i
	lastSigners       map[int]struct{}   // maps the signers in the latest call to aggregate keys
	lastAggregatedKey crypto.PublicKey   // the latest aggregated public key
	sync.RWMutex                         // the above "latest" data only make sense in a concurrent safe model, the lock maintains the thread-safety
	// since the caller should not be aware of the internal non thread-safe algorithm.
}

// creates a new public key aggregator from all possible public keys
func NewPublicKeyAggregator(publicKeys []crypto.PublicKey) (*PublicKeyAggregator, error) {
	// check for BLS keys
	for i, key := range publicKeys {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, engine.NewInvalidInputErrorf("key at index %d is not a BLS key", i)
		}
	}
	aggregator := &PublicKeyAggregator{
		n:                 len(publicKeys),
		publicKeys:        publicKeys,
		lastSigners:       make(map[int]struct{}),
		lastAggregatedKey: crypto.NeutralBLSPublicKey(),
		RWMutex:           sync.RWMutex{},
	}
	return aggregator, nil
}

// KeyAggregate returns the aggregated public key of the input signers.
func (p *PublicKeyAggregator) KeyAggregate(signers []int) (crypto.PublicKey, error) {

	// this greedy algorithm assumes the signers set does not vary much from one call
	// to KeyAggregate to another. It computes the delta of signers compared to the
	// latest list of signers and adjust the latest aggregated public key. This is faster
	// than aggregating the public keys from scratch at each call.

	// read lock to read consistent last key and last signers
	p.RLock()
	// get the signers delta and update the last list for the next comparison
	newSignerKeys, missingSignerKeys, updatedSignerSet := p.deltaKeys(signers)
	lastKey := p.lastAggregatedKey
	p.RUnlock()

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
	p.Lock()
	p.lastSigners = updatedSignerSet
	p.lastAggregatedKey = updatedKey
	p.Unlock()
	return updatedKey, nil
}

// keysDelta computes the delta between the reference s.lastSigners
// and the input identity list.
// It returns a list of the new signer keys, a list of the missing signer keys and the new map of signers.
func (p *PublicKeyAggregator) deltaKeys(signers []int) (
	[]crypto.PublicKey, []crypto.PublicKey, map[int]struct{}) {

	var newSignerKeys, missingSignerKeys []crypto.PublicKey

	// create a map of the input list,
	// and check the new signers
	signersMap := make(map[int]struct{})
	for _, signer := range signers {
		signersMap[signer] = struct{}{}
		_, ok := p.lastSigners[signer]
		if !ok {
			newSignerKeys = append(newSignerKeys, p.publicKeys[signer])
		}
	}

	// look for missing signers
	for signer := range p.lastSigners {
		_, ok := signersMap[signer]
		if !ok {
			missingSignerKeys = append(missingSignerKeys, p.publicKeys[signer])
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
