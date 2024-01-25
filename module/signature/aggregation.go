package signature

import (
	"fmt"
	"sync"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
)

// SignatureAggregatorSameMessage aggregates BLS signatures of the same message from different signers.
// The public keys and message are agreed upon upfront.
//
// Currently, the module does not support signatures with multiplicity higher than 1. Each signer is allowed
// to sign at most once.
//
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module is only safe under the assumption that all proofs of possession (PoP) of the public keys
// are valid.
//
// Implementation of SignatureAggregator is not thread-safe, the caller should
// make sure the calls are concurrent safe.
type SignatureAggregatorSameMessage struct {
	message          []byte
	hasher           hash.Hasher
	n                int                // number of participants indexed from 0 to n-1
	publicKeys       []crypto.PublicKey // keys indexed from 0 to n-1, signer i is assigned to public key i
	indexToSignature map[int]string     // signatures indexed by the signer index

	// To remove overhead from repeated Aggregate() calls, we cache the aggregation result.
	// Whenever a new signature is added, we reset `cachedSignature` to nil.
	cachedSignature     crypto.Signature // cached raw aggregated signature
	cachedSignerIndices []int            // cached indices of signers that contributed to `cachedSignature`
}

// NewSignatureAggregatorSameMessage returns a new SignatureAggregatorSameMessage structure.
//
// A new SignatureAggregatorSameMessage is needed for each set of public keys. If the key set changes,
// a new structure needs to be instantiated. Participants are defined by their public keys, and are
// indexed from 0 to n-1 where n is the length of the public key slice.
// The aggregator does not verify PoPs of input public keys, it assumes verification was done outside
// this module.
// The constructor errors if:
//   - length of keys is zero
//   - any input public key is not a BLS 12-381 key
func NewSignatureAggregatorSameMessage(
	message []byte, // message to be aggregate signatures for
	dsTag string, // domain separation tag used for signatures
	publicKeys []crypto.PublicKey, // public keys of participants agreed upon upfront
) (*SignatureAggregatorSameMessage, error) {

	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("number of participants must be larger than 0, got %d", len(publicKeys))
	}
	// sanity check for BLS keys
	for i, key := range publicKeys {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, fmt.Errorf("key at index %d is not a BLS key", i)
		}
	}

	return &SignatureAggregatorSameMessage{
		message:          message,
		hasher:           NewBLSHasher(dsTag),
		n:                len(publicKeys),
		publicKeys:       publicKeys,
		indexToSignature: make(map[int]string),
		cachedSignature:  nil,
	}, nil
}

// Verify verifies the input signature under the stored message and stored
// key at the input index.
//
// This function does not update the internal state.
// The function errors:
//   - InvalidSignerIdxError if the signer index is out of bound
//   - generic error for unexpected runtime failures
//
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) Verify(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, NewInvalidSignerIdxErrorf("signer index %d is invalid", signer)
	}
	return s.publicKeys[signer].Verify(sig, s.message, s.hasher)
}

// VerifyAndAdd verifies the input signature under the stored message and stored
// key at the input index. If the verification passes, the signature is added to the internal
// signature state.
// The function errors:
//   - InvalidSignerIdxError if the signer index is out of bound
//   - DuplicatedSignerIdxError if a signature from the same signer index has already been added
//   - generic error for unexpected runtime failures
//
// The function does not return an error for any invalid signature.
// If any error is returned, the returned bool is false.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) VerifyAndAdd(signer int, sig crypto.Signature) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, NewInvalidSignerIdxErrorf("signer index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return false, NewDuplicatedSignerIdxErrorf("signature from signer index %d has already been added", signer)
	}
	// signature is new
	ok, err := s.publicKeys[signer].Verify(sig, s.message, s.hasher) // no errors expected
	if ok {
		s.add(signer, sig)
	}
	return ok, err
}

// adds signature and assumes `signer` is valid
func (s *SignatureAggregatorSameMessage) add(signer int, sig crypto.Signature) {
	s.cachedSignature = nil
	s.indexToSignature[signer] = string(sig)
}

// TrustedAdd adds a signature to the internal state without verifying it.
//
// The Aggregate function makes a sanity check on the aggregated signature and only
// outputs valid signatures. This would detect if TrustedAdd has added any invalid
// signature.
// The function errors:
//   - InvalidSignerIdxError if the signer index is out of bound
//   - DuplicatedSignerIdxError if a signature from the same signer index has already been added
//
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) TrustedAdd(signer int, sig crypto.Signature) error {
	if signer >= s.n || signer < 0 {
		return NewInvalidSignerIdxErrorf("signer index %d is invalid", signer)
	}
	_, duplicate := s.indexToSignature[signer]
	if duplicate {
		return NewDuplicatedSignerIdxErrorf("signature from signer index %d has already been added", signer)
	}
	// signature is new
	s.add(signer, sig)
	return nil
}

// HasSignature checks if a signer has already provided a valid signature.
// The function errors:
//   - InvalidSignerIdxError if the signer index is out of bound
//
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) HasSignature(signer int) (bool, error) {
	if signer >= s.n || signer < 0 {
		return false, NewInvalidSignerIdxErrorf("signer index %d is invalid", signer)
	}
	_, ok := s.indexToSignature[signer]
	return ok, nil
}

// Aggregate aggregates the added BLS signatures and returns the aggregated signature.
//
// The function errors if any signature fails the deserialization. It also performs a final
// verification and errors if the aggregated signature is invalid.
// It also errors if no signatures were added.
// Post-check of aggregated signature is required for function safety, as `TrustedAdd` allows
// adding invalid signatures or signatures that yield the identity aggregate. In both failure
// cases, the function discards the generated aggregate and errors.
// The function is not thread-safe.
// Returns:
//   - InsufficientSignaturesError if no signatures have been added yet
//   - InvalidSignatureIncludedError if:
//     -- some signature(s), included via TrustedAdd, fail to deserialize (regardless of the aggregated public key)
//     -- Or all signatures deserialize correctly but some signature(s), included via TrustedAdd, are
//     invalid (while aggregated public key is valid)
//   - ErrIdentityPublicKey if the signer's public keys add up to the BLS identity public key.
//     Any aggregated signature would fail the cryptographic verification if verified against the
//     the identity public key. This case can only happen if public keys were forged to sum up to
//     an identity public key. Under the assumption that PoPs of all keys are valid, an identity
//     public key can only happen if all private keys (and hence their corresponding public keys)
//     have been generated by colluding participants.
func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	// check if signature was already computed
	if s.cachedSignature != nil {
		return s.cachedSignerIndices, s.cachedSignature, nil
	}

	// compute aggregation result and cache it in `s.cachedSignerIndices`, `s.cachedSignature`
	sharesNum := len(s.indexToSignature)
	indices := make([]int, 0, sharesNum)
	signatures := make([]crypto.Signature, 0, sharesNum)
	for i, sig := range s.indexToSignature {
		indices = append(indices, i)
		signatures = append(signatures, []byte(sig))
	}

	aggregatedSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		// an empty list of signatures is not allowed
		if crypto.IsBLSAggregateEmptyListError(err) {
			return nil, nil, NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures: %w", err)
		}
		// invalid signature serialization, regardless of the signer's public key
		if crypto.IsInvalidSignatureError(err) {
			return nil, nil, NewInvalidSignatureIncludedErrorf("signatures with invalid structure were included via TrustedAdd: %w", err)
		}
		return nil, nil, fmt.Errorf("BLS signature aggregation failed: %w", err)
	}

	ok, aggregatedKey, err := s.VerifyAggregate(indices, aggregatedSignature) // no errors expected (unless some public BLS keys are invalid)
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected error during signature aggregation: %w", err)
	}
	if !ok {
		// check for identity aggregated key (invalid aggregated signature)
		if aggregatedKey.Equals(crypto.IdentityBLSPublicKey()) {
			return nil, nil, fmt.Errorf("invalid aggregated signature: %w", ErrIdentityPublicKey)
		}
		// this case can only happen if at least one added signature via TrustedAdd does not verify against
		// the signer's corresponding public key
		return nil, nil, NewInvalidSignatureIncludedErrorf("invalid signature(s) have been included via TrustedAdd")
	}
	s.cachedSignature = aggregatedSignature
	s.cachedSignerIndices = indices
	return indices, aggregatedSignature, nil
}

// VerifyAggregate verifies an input signature against the stored message and the stored
// keys corresponding to the input signers.
// The aggregated public key of input signers is returned. In particular this allows comparing the
// aggregated key against the identity public key.
// The function is not thread-safe.
// Possible returns:
//   - (true, agg_key, nil): signature is valid
//   - (false, agg_key, nil): signature is cryptographically invalid. This also includes the case where
//     `agg_key` is equal to the identity public key (because of equivocation). If the caller needs to
//     differentiate this case, `crypto.IsIdentityPublicKey` can be used to test the returned `agg_key`
//   - (false, nil, err) with error types:
//     -- InsufficientSignaturesError if no signer indices are given (`signers` is empty)
//     -- InvalidSignerIdxError if some signer indices are out of bound
//     -- generic error in case of an unexpected runtime failure
func (s *SignatureAggregatorSameMessage) VerifyAggregate(signers []int, sig crypto.Signature) (bool, crypto.PublicKey, error) {
	keys := make([]crypto.PublicKey, 0, len(signers))
	for _, signer := range signers {
		if signer >= s.n || signer < 0 {
			return false, nil, NewInvalidSignerIdxErrorf("signer index %d is invalid", signer)
		}
		keys = append(keys, s.publicKeys[signer])
	}

	aggregatedKey, err := crypto.AggregateBLSPublicKeys(keys)
	if err != nil {
		// error for:
		//  * empty `keys` slice results in crypto.blsAggregateEmptyListError
		//  * some keys are not BLS12 381 keys, which should not happen, as we checked
		//    each key's signing algorithm in the constructor to be `crypto.BLSBLS12381`
		if crypto.IsBLSAggregateEmptyListError(err) {
			return false, nil, NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures: %w", err)
		}
		return false, nil, fmt.Errorf("unexpected internal error during public key aggregation: %w", err)
	}
	ok, err := aggregatedKey.Verify(sig, s.message, s.hasher) // no errors expected
	if err != nil {
		return false, nil, fmt.Errorf("signature verification failed: %w", err)
	}
	return ok, aggregatedKey, nil
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

// NewPublicKeyAggregator creates an index-based key aggregator, for the given list of authorized signers.
//
// The constructor errors if:
//   - the input keys are empty.
//   - any input public key algorithm is not BLS.
func NewPublicKeyAggregator(publicKeys []crypto.PublicKey) (*PublicKeyAggregator, error) {
	// check for empty list
	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("input keys cannot be empty")
	}
	// check for BLS keys
	for i, key := range publicKeys {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, fmt.Errorf("key at index %d is not a BLS key", i)
		}
	}
	aggregator := &PublicKeyAggregator{
		n:                 len(publicKeys),
		publicKeys:        publicKeys,
		lastSigners:       make(map[int]struct{}),
		lastAggregatedKey: crypto.IdentityBLSPublicKey(),
		RWMutex:           sync.RWMutex{},
	}
	return aggregator, nil
}

// KeyAggregate returns the aggregated public key of the input signers.
//
// The aggregation errors if:
//   - generic error if input signers is empty.
//   - InvalidSignerIdxError if any signer is out of bound.
//   - other generic errors are unexpected during normal operations.
func (p *PublicKeyAggregator) KeyAggregate(signers []int) (crypto.PublicKey, error) {
	// check for empty list
	if len(signers) == 0 {
		return nil, fmt.Errorf("input signers cannot be empty")
	}

	// check signers
	for i, signer := range signers {
		if signer >= p.n || signer < 0 {
			return nil, NewInvalidSignerIdxErrorf("signer %d at index %d is invalid", signer, i)
		}
	}

	// this greedy algorithm assumes the signers set does not vary much from one call
	// to KeyAggregate to another. It computes the delta of signers compared to the
	// latest list of signers and adjust the latest aggregated public key. This is faster
	// than aggregating the public keys from scratch at each call.

	// read lock to read consistent last key and last signers
	p.RLock()
	// get the signers delta and update the last list for the next comparison
	addedSignerKeys, missingSignerKeys, updatedSignerSet := p.deltaKeys(signers)
	lastKey := p.lastAggregatedKey
	p.RUnlock()

	// checks whether the delta of signers is larger than new list of signers.
	deltaIsLarger := len(addedSignerKeys)+len(missingSignerKeys) > len(updatedSignerSet)

	var updatedKey crypto.PublicKey
	var err error
	if deltaIsLarger {
		// it is faster to aggregate the keys from scratch in this case
		newSigners := make([]crypto.PublicKey, 0, len(updatedSignerSet))
		for signer := range updatedSignerSet {
			newSigners = append(newSigners, p.publicKeys[signer])
		}
		updatedKey, err = crypto.AggregateBLSPublicKeys(newSigners)
		if err != nil {
			// not expected as the keys are not empty and all keys are BLS
			return nil, fmt.Errorf("aggregating keys failed: %w", err)
		}
	} else {
		// it is faster to adjust the existing aggregated key in this case
		// add the new keys
		updatedKey, err = crypto.AggregateBLSPublicKeys(append(addedSignerKeys, lastKey))
		if err != nil {
			// no error expected as there is at least one key (from the `append`), and all keys are BLS (checked in the constructor)
			return nil, fmt.Errorf("adding new keys failed: %w", err)
		}
		// remove the missing keys
		updatedKey, err = crypto.RemoveBLSPublicKeys(updatedKey, missingSignerKeys)
		if err != nil {
			// no error expected as all keys are BLS (checked in the constructor)
			return nil, fmt.Errorf("removing missing keys failed: %w", err)
		}
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

	var addedSignerKeys, missingSignerKeys []crypto.PublicKey

	// create a map of the input list,
	// and check the new signers
	newSignersMap := make(map[int]struct{})
	for _, signer := range signers {
		newSignersMap[signer] = struct{}{}
		_, ok := p.lastSigners[signer]
		if !ok {
			addedSignerKeys = append(addedSignerKeys, p.publicKeys[signer])
		}
	}

	// look for missing signers
	for signer := range p.lastSigners {
		_, ok := newSignersMap[signer]
		if !ok {
			missingSignerKeys = append(missingSignerKeys, p.publicKeys[signer])
		}
	}
	return addedSignerKeys, missingSignerKeys, newSignersMap
}
